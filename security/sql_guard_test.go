package security

import (
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{"valid_lowercase", "user_id", false},
		{"valid_uppercase", "USER_ID", false},
		{"valid_mixed", "userId", false},
		{"valid_with_numbers", "user_123", false},
		{"empty", "", true},
		{"too_long", string(make([]byte, 65)), true},
		{"with_dash", "user-id", true},
		{"with_space", "user id", true},
		{"with_dot", "user.id", true},
		{"sql_keyword_SELECT", "SELECT", true},
		{"sql_keyword_DROP", "DROP", true},
		{"starts_with_number", "123user", true},
		{"chinese_chars", "用戶ID", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateIdentifier(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIdentifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFilePath(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid_relative", "backup.db", false},
		{"valid_subdirectory", "backups/db.sqlite", false},
		{"empty", "", true},
		{"path_traversal", "../etc/passwd", true},
		{"path_traversal_hidden", "backup/../../../etc/passwd", true},
		{"absolute_path", "/var/lib/db.sqlite", true},
		{"tmp_absolute_allowed", "/tmp/backup.db", false},
		{"with_semicolon", "backup;rm -rf", true},
		{"with_pipe", "backup|cat", true},
		{"with_backtick", "backup`id`", true},
		{"with_newline", "backup\n.db", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := guard.SanitizeFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLikePattern(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"empty", "", false},
		{"simple", "test", false},
		{"with_wildcard_percent", "test%", false},
		{"with_wildcard_underscore", "t_st", false},
		{"with_single_quote", "test'", true},
		{"with_double_quote", "test\"", true},
		{"with_semicolon", "test;", true},
		{"with_comment", "test--", true},
		{"with_block_comment", "test/*", true},
		{"too_long", string(make([]byte, 257)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateLikePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLikePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEscapeLikePattern(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{"no_special_chars", "test", "test"},
		{"with_percent", "test%", "test\\%"},
		{"with_underscore", "t_st", "t\\_st"},
		{"with_backslash", "test\\", "test\\\\"},
		{"multiple_special", "t%e_s\\t", "t\\%e\\_s\\\\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guard.EscapeLikePattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("EscapeLikePattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateOrderByColumn(t *testing.T) {
	guard := NewSQLGuard()

	allowed := []string{"id", "name", "created_at", "updated_at"}

	tests := []struct {
		name    string
		column  string
		wantErr bool
	}{
		{"valid_in_whitelist", "id", false},
		{"valid_name", "name", false},
		{"not_in_whitelist", "password", true},
		{"empty", "", true},
		{"sql_injection_attempt", "id; DROP TABLE users", true},
		{"sql_keyword", "SELECT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateOrderByColumn(tt.column, allowed)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrderByColumn() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLimit(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{"valid_small", 10, false},
		{"valid_large", 1000, false},
		{"valid_max", 10000, false},
		{"negative", -1, true},
		{"too_large", 10001, true},
		{"zero", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateLimit(tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOffset(t *testing.T) {
	guard := NewSQLGuard()

	tests := []struct {
		name    string
		offset  int
		wantErr bool
	}{
		{"valid_small", 0, false},
		{"valid_large", 1000, false},
		{"valid_max", 1000000, false},
		{"negative", -1, true},
		{"too_large", 1000001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.ValidateOffset(tt.offset)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOffset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 基準測試
func BenchmarkValidateIdentifier(b *testing.B) {
	guard := NewSQLGuard()
	for i := 0; i < b.N; i++ {
		guard.ValidateIdentifier("user_id")
	}
}

func BenchmarkSanitizeFilePath(b *testing.B) {
	guard := NewSQLGuard()
	for i := 0; i < b.N; i++ {
		guard.SanitizeFilePath("backups/db.sqlite")
	}
}

func BenchmarkEscapeLikePattern(b *testing.B) {
	guard := NewSQLGuard()
	for i := 0; i < b.N; i++ {
		guard.EscapeLikePattern("test%pattern_with\\special")
	}
}
