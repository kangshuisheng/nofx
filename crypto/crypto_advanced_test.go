package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHybridEncryptionFlow tests the complete RSA + AES-GCM encryption flow
func TestHybridEncryptionFlow(t *testing.T) {
	// Setup: Create crypto service with data encryption key
	tmpKeyPath := filepath.Join(os.TempDir(), "test_hybrid_key.pem")
	defer os.Remove(tmpKeyPath)

	// Set environment variable for data encryption
	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		aadParts  []string
	}{
		{
			name:      "Simple string",
			plaintext: "Hello, World!",
			aadParts:  []string{"user123", "session456"},
		},
		{
			name:      "API key",
			plaintext: "sk_test_1234567890abcdefghijklmnop",
			aadParts:  []string{"user", "api_key"},
		},
		{
			name:      "Long text",
			plaintext: strings.Repeat("LongText", 1000),
			aadParts:  []string{},
		},
		{
			name:      "Special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;:',.<>?/`~",
			aadParts:  []string{"special"},
		},
		{
			name:      "Unicode characters",
			plaintext: "こんにちは世界 🌍",
			aadParts:  []string{},
		},
		{
			name:      "Empty string",
			plaintext: "",
			aadParts:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := cs.EncryptForStorage(tt.plaintext, tt.aadParts...)
			if err != nil {
				if tt.plaintext == "" {
					// Empty string should return empty without error
					if encrypted != "" {
						t.Errorf("Empty plaintext should return empty string")
					}
					return
				}
				t.Fatalf("EncryptForStorage failed: %v", err)
			}

			// Verify encrypted format
			if tt.plaintext != "" {
				if !strings.HasPrefix(encrypted, "ENC:v1:") {
					t.Errorf("Encrypted value should have ENC:v1: prefix, got: %s", encrypted[:20])
				}
			}

			// Decrypt
			decrypted, err := cs.DecryptFromStorage(encrypted, tt.aadParts...)
			if err != nil {
				t.Fatalf("DecryptFromStorage failed: %v", err)
			}

			// Verify match
			if decrypted != tt.plaintext {
				t.Errorf("Decrypted value doesn't match original.\nWant: %s\nGot:  %s",
					tt.plaintext[:min(50, len(tt.plaintext))],
					decrypted[:min(50, len(decrypted))])
			}

			t.Logf("✅ Successfully encrypted and decrypted: %s", tt.name)
		})
	}
}

// TestEncryptionWithWrongAAD tests that decryption fails with incorrect AAD
func TestEncryptionWithWrongAAD(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_aad_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	plaintext := "Sensitive data"
	correctAAD := []string{"user123", "session456"}

	// Encrypt with correct AAD
	encrypted, err := cs.EncryptForStorage(plaintext, correctAAD...)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with wrong AAD
	wrongAAD := []string{"user999", "session999"}
	_, err = cs.DecryptFromStorage(encrypted, wrongAAD...)
	if err == nil {
		t.Error("Decryption with wrong AAD should fail")
	}

	// Decrypt with correct AAD should work
	decrypted, err := cs.DecryptFromStorage(encrypted, correctAAD...)
	if err != nil {
		t.Fatalf("Decryption with correct AAD failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted value doesn't match original")
	}

	t.Log("✅ AAD validation working correctly")
}

// TestDoubleEncryption tests that encrypting already encrypted data doesn't re-encrypt
func TestDoubleEncryption(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_double_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	plaintext := "Test data"

	// First encryption
	encrypted1, err := cs.EncryptForStorage(plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	// Second encryption (should return the same value)
	encrypted2, err := cs.EncryptForStorage(encrypted1)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	if encrypted1 != encrypted2 {
		t.Error("Double encryption should return the same encrypted value")
	}

	// Decrypt should still work
	decrypted, err := cs.DecryptFromStorage(encrypted2)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted value doesn't match original")
	}

	t.Log("✅ Double encryption protection working")
}

// TestKeyGeneration tests RSA key pair generation
func TestKeyGeneration(t *testing.T) {
	// Skip if DATA_ENCRYPTION_KEY not set
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		t.Skip("Skipping: DATA_ENCRYPTION_KEY not set")
	}

	tmpKeyPath := filepath.Join(os.TempDir(), "test_gen_key.pem")
	defer os.Remove(tmpKeyPath)

	// Generate key pair
	err := GenerateRSAKeyPair(tmpKeyPath)
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	// Verify private key file exists
	if _, err := os.Stat(tmpKeyPath); os.IsNotExist(err) {
		t.Error("Private key file was not created")
	}

	// Verify public key file exists
	publicKeyPath := strings.TrimSuffix(tmpKeyPath, ".pem") + ".pub"
	defer os.Remove(publicKeyPath)

	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		t.Error("Public key file was not created")
	}

	// Load the generated key
	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to load generated key: %v", err)
	}

	// Verify key can be used for encryption
	if cs.publicKey == nil {
		t.Error("Public key is nil")
	}

	if cs.privateKey == nil {
		t.Error("Private key is nil")
	}

	t.Log("✅ Key generation and loading successful")
}

// TestDataKeyVariousFormats tests loading data encryption key in different formats
func TestDataKeyVariousFormats(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_datakey_key.pem")
	defer os.Remove(tmpKeyPath)

	// Generate RSA key first
	err := GenerateRSAKeyPair(tmpKeyPath)
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	tests := []struct {
		name       string
		keyValue   string
		shouldWork bool
	}{
		{
			name:       "Base64 encoded 32-byte key",
			keyValue:   base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")),
			shouldWork: true,
		},
		{
			name:       "Plain text (will be hashed)",
			keyValue:   "my-secret-key-password",
			shouldWork: true,
		},
		{
			name:       "Hex encoded key",
			keyValue:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldWork: true,
		},
		{
			name:       "Empty key",
			keyValue:   "",
			shouldWork: false,
		},
		{
			name:       "Short key (will be hashed)",
			keyValue:   "short",
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DATA_ENCRYPTION_KEY", tt.keyValue)
			defer os.Unsetenv("DATA_ENCRYPTION_KEY")

			cs, err := NewCryptoService(tmpKeyPath)

			if tt.shouldWork {
				if err != nil {
					t.Errorf("Expected key to work, but got error: %v", err)
				} else {
					if !cs.HasDataKey() {
						t.Error("Data key should be loaded")
					}
					t.Logf("✅ Key format accepted: %s", tt.name)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for invalid key, but got none")
				} else {
					t.Logf("✅ Invalid key rejected: %s", tt.name)
				}
			}
		})
	}
}

// TestConcurrentEncryption tests encryption under concurrent access
func TestConcurrentEncryption(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_concurrent_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	numGoroutines := 50
	numOperations := 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				plaintext := "Test data " + string(rune('A'+id)) + string(rune('0'+j))

				// Encrypt
				encrypted, err := cs.EncryptForStorage(plaintext)
				if err != nil {
					errors <- err
					continue
				}

				// Decrypt
				decrypted, err := cs.DecryptFromStorage(encrypted)
				if err != nil {
					errors <- err
					continue
				}

				// Verify
				if decrypted != plaintext {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent operation error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Encountered %d errors in concurrent operations", errorCount)
	} else {
		t.Logf("✅ %d concurrent encrypt/decrypt operations completed successfully",
			numGoroutines*numOperations*2)
	}
}

// TestLargeDataEncryption tests encryption of large data
func TestLargeDataEncryption(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_large_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	// Test with various sizes
	sizes := []int{
		1 * 1024,        // 1 KB
		10 * 1024,       // 10 KB
		100 * 1024,      // 100 KB
		1 * 1024 * 1024, // 1 MB
	}

	for _, size := range sizes {
		t.Run("Size_"+string(rune(size)), func(t *testing.T) {
			// Generate random data
			plaintext := make([]byte, size)
			_, err := rand.Read(plaintext)
			if err != nil {
				t.Fatalf("Failed to generate random data: %v", err)
			}

			plaintextStr := base64.StdEncoding.EncodeToString(plaintext)

			start := time.Now()

			// Encrypt
			encrypted, err := cs.EncryptForStorage(plaintextStr)
			if err != nil {
				t.Fatalf("Encryption failed for size %d: %v", size, err)
			}

			encryptTime := time.Since(start)

			// Decrypt
			start = time.Now()
			decrypted, err := cs.DecryptFromStorage(encrypted)
			if err != nil {
				t.Fatalf("Decryption failed for size %d: %v", size, err)
			}

			decryptTime := time.Since(start)

			// Verify
			if decrypted != plaintextStr {
				t.Errorf("Decrypted data doesn't match for size %d", size)
			}

			t.Logf("✅ Size: %d bytes - Encrypt: %v, Decrypt: %v",
				size, encryptTime, decryptTime)
		})
	}
}

// TestEncryptionWithoutDataKey tests behavior when data key is not set
func TestEncryptionWithoutDataKey(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_nokey_key.pem")
	defer os.Remove(tmpKeyPath)

	// Generate RSA key first
	err := GenerateRSAKeyPair(tmpKeyPath)
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	// Make sure DATA_ENCRYPTION_KEY is not set
	os.Unsetenv("DATA_ENCRYPTION_KEY")

	// Should fail to create crypto service
	_, err = NewCryptoService(tmpKeyPath)
	if err == nil {
		t.Error("Expected error when DATA_ENCRYPTION_KEY is not set")
	}

	if !strings.Contains(err.Error(), "DATA_ENCRYPTION_KEY") {
		t.Errorf("Error should mention DATA_ENCRYPTION_KEY, got: %v", err)
	}

	t.Log("✅ Correctly rejects missing data encryption key")
}

// TestPublicKeyExport tests exporting public key in PEM format
func TestPublicKeyExport(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_export_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	publicKeyPEM := cs.GetPublicKeyPEM()

	if publicKeyPEM == "" {
		t.Error("Public key PEM is empty")
	}

	if !strings.Contains(publicKeyPEM, "BEGIN PUBLIC KEY") {
		t.Error("Public key PEM doesn't contain proper header")
	}

	if !strings.Contains(publicKeyPEM, "END PUBLIC KEY") {
		t.Error("Public key PEM doesn't contain proper footer")
	}

	// Verify it's valid base64 content
	lines := strings.Split(publicKeyPEM, "\n")
	if len(lines) < 3 {
		t.Error("Public key PEM has too few lines")
	}

	t.Logf("✅ Public key exported successfully (%d bytes)", len(publicKeyPEM))
}

// TestEncryptionConsistency tests that same input produces different ciphertext (due to random IV)
func TestEncryptionConsistency(t *testing.T) {
	tmpKeyPath := filepath.Join(os.TempDir(), "test_consistency_key.pem")
	defer os.Remove(tmpKeyPath)

	os.Setenv("DATA_ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes")
	defer os.Unsetenv("DATA_ENCRYPTION_KEY")

	cs, err := NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Fatalf("Failed to create crypto service: %v", err)
	}

	plaintext := "Consistent test data"

	// Encrypt the same plaintext multiple times
	encrypted1, err := cs.EncryptForStorage(plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	encrypted2, err := cs.EncryptForStorage(plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// Ciphertexts should be different (due to random IV)
	if encrypted1 == encrypted2 {
		t.Error("Same plaintext should produce different ciphertext (random IV)")
	}

	// But both should decrypt to the same value
	decrypted1, err := cs.DecryptFromStorage(encrypted1)
	if err != nil {
		t.Fatalf("First decryption failed: %v", err)
	}

	decrypted2, err := cs.DecryptFromStorage(encrypted2)
	if err != nil {
		t.Fatalf("Second decryption failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Decrypted values don't match original")
	}

	t.Log("✅ Encryption produces unique ciphertext with random IV")
}

// min function is defined in encryption_test.go
