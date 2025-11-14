package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SQLGuard SQL 注入防護工具
type SQLGuard struct{}

// NewSQLGuard 創建 SQL 防護實例
func NewSQLGuard() *SQLGuard {
	return &SQLGuard{}
}

// ValidateIdentifier 驗證 SQL 標識符（表名、列名等）
// 只允許字母、數字、下劃線
func (g *SQLGuard) ValidateIdentifier(identifier string) error {
	if identifier == "" {
		return fmt.Errorf("標識符不能為空")
	}

	// 長度限制（防止 DoS）
	if len(identifier) > 64 {
		return fmt.Errorf("標識符長度超過限制（最大64字符）")
	}

	// 只允許字母、數字、下劃線
	validPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !validPattern.MatchString(identifier) {
		return fmt.Errorf("標識符包含非法字符：%s", identifier)
	}

	// 黑名單檢查：SQL 關鍵字
	sqlKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE",
		"ALTER", "EXEC", "EXECUTE", "UNION", "OR", "AND",
	}
	upperIdentifier := strings.ToUpper(identifier)
	for _, keyword := range sqlKeywords {
		if upperIdentifier == keyword {
			return fmt.Errorf("標識符不能使用 SQL 關鍵字：%s", identifier)
		}
	}

	return nil
}

// SanitizeFilePath 清理文件路徑，防止路徑遍歷攻擊
func (g *SQLGuard) SanitizeFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("路徑不能為空")
	}

	// 移除多餘的空格
	path = strings.TrimSpace(path)

	// 檢查路徑遍歷嘗試
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("路徑包含非法字符：..")
	}

	// 禁止絕對路徑（如果需要絕對路徑，應該顯式允許）
	if filepath.IsAbs(path) && !strings.HasPrefix(path, "/tmp/") {
		// 只允許 /tmp/ 目錄的絕對路徑（可根據需求調整）
		return "", fmt.Errorf("不允許使用絕對路徑")
	}

	// 清理路徑（移除 ./ 和多餘的斜杠）
	cleanPath := filepath.Clean(path)

	// 檢查危險字符
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(cleanPath, char) {
			return "", fmt.Errorf("路徑包含危險字符：%s", char)
		}
	}

	return cleanPath, nil
}

// ValidateLikePattern 驗證 LIKE 模式，防止注入
func (g *SQLGuard) ValidateLikePattern(pattern string) error {
	if pattern == "" {
		return nil // 空模式允許
	}

	// 長度限制
	if len(pattern) > 256 {
		return fmt.Errorf("LIKE 模式長度超過限制（最大256字符）")
	}

	// 檢查是否包含危險字符（除了 % 和 _ 是 LIKE 的通配符）
	dangerousChars := []string{"'", "\"", ";", "--", "/*", "*/"}
	for _, char := range dangerousChars {
		if strings.Contains(pattern, char) {
			return fmt.Errorf("LIKE 模式包含危險字符：%s", char)
		}
	}

	return nil
}

// EscapeLikePattern 轉義 LIKE 模式中的特殊字符
// 用戶輸入的 % 和 _ 應該被轉義為字面量
func (g *SQLGuard) EscapeLikePattern(pattern string) string {
	// 轉義反斜杠（轉義符本身）
	escaped := strings.ReplaceAll(pattern, "\\", "\\\\")
	// 轉義 % 和 _
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	return escaped
}

// ValidateOrderByColumn 驗證 ORDER BY 列名
// ORDER BY 不能使用參數化查詢，需要額外驗證
func (g *SQLGuard) ValidateOrderByColumn(column string, allowedColumns []string) error {
	if column == "" {
		return fmt.Errorf("列名不能為空")
	}

	// 驗證標識符格式
	if err := g.ValidateIdentifier(column); err != nil {
		return err
	}

	// 白名單檢查
	if len(allowedColumns) > 0 {
		found := false
		for _, allowed := range allowedColumns {
			if column == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("列名不在允許列表中：%s", column)
		}
	}

	return nil
}

// ValidateLimit 驗證 LIMIT 值
func (g *SQLGuard) ValidateLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("LIMIT 不能為負數")
	}
	if limit > 10000 {
		return fmt.Errorf("LIMIT 超過最大值（10000）")
	}
	return nil
}

// ValidateOffset 驗證 OFFSET 值
func (g *SQLGuard) ValidateOffset(offset int) error {
	if offset < 0 {
		return fmt.Errorf("OFFSET 不能為負數")
	}
	if offset > 1000000 {
		return fmt.Errorf("OFFSET 超過最大值（1000000）")
	}
	return nil
}
