package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"nofx/crypto"
)

func newTestCryptoHandler(t *testing.T, allow bool) *CryptoHandler {
	t.Helper()
	t.Setenv("DATA_ENCRYPTION_KEY", "unit-test-key")
	svc, err := crypto.NewCryptoService("crypto/test_rsa_key.pem")
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}
	return NewCryptoHandler(svc, allow)
}

func TestCryptoHandlerDecryptDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestCryptoHandler(t, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/crypto/decrypt", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.HandleDecryptSensitiveData(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when decrypt API disabled, got %d", w.Code)
	}
}

func TestCryptoHandlerDecryptRequiresAAD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestCryptoHandler(t, true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "user-123")

	body := `{"wrappedKey":"","iv":"","ciphertext":"","aad":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/crypto/decrypt", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.HandleDecryptSensitiveData(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing AAD metadata, got %d", w.Code)
	}
}
