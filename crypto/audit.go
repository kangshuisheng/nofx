package crypto

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent 审计事件
type AuditEvent struct {
	Timestamp string `json:"timestamp"`
	UserID    string `json:"user_id"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Result    string `json:"result"` // "success" or "failure"
	IPAddress string `json:"ip_address,omitempty"`
	Details   string `json:"details,omitempty"`
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu       sync.Mutex
	filePath string
	enabled  bool
}

var (
	auditLogger     *AuditLogger
	auditLoggerOnce sync.Once
)

// GetAuditLogger 获取审计日志记录器（单例）
func GetAuditLogger() *AuditLogger {
	auditLoggerOnce.Do(func() {
		// 默认启用审计日志
		enabled := os.Getenv("AUDIT_LOG_ENABLED") != "false"

		// 审计日志目录
		logDir := os.Getenv("AUDIT_LOG_DIR")
		if logDir == "" {
			logDir = "logs/audit"
		}

		// 创建日志目录
		if err := os.MkdirAll(logDir, 0700); err != nil {
			log.Printf("⚠️ 创建审计日志目录失败: %v", err)
			enabled = false
		}

		// 日志文件路径（按日期分割）
		filename := time.Now().Format("2006-01-02") + ".jsonl"
		filePath := filepath.Join(logDir, filename)

		auditLogger = &AuditLogger{
			filePath: filePath,
			enabled:  enabled,
		}

		if enabled {
			log.Printf("📋 审计日志已启用: %s", filePath)
		}
	})
	return auditLogger
}

// Log 记录审计事件
func (al *AuditLogger) Log(event AuditEvent) {
	if !al.enabled {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	// 设置时间戳
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// 序列化为 JSON
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("⚠️ 序列化审计事件失败: %v", err)
		return
	}

	// 追加到文件
	f, err := os.OpenFile(al.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("⚠️ 打开审计日志文件失败: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		log.Printf("⚠️ 写入审计日志失败: %v", err)
	}
}

// LogDecryption 记录解密操作
func (al *AuditLogger) LogDecryption(userID, resource, result string) {
	al.Log(AuditEvent{
		UserID:   userID,
		Action:   "decrypt",
		Resource: resource,
		Result:   result,
	})
}

// LogEncryption 记录加密操作
func (al *AuditLogger) LogEncryption(userID, resource, result string) {
	al.Log(AuditEvent{
		UserID:   userID,
		Action:   "encrypt",
		Resource: resource,
		Result:   result,
	})
}

// LogKeyAccess 记录密钥访问
func (al *AuditLogger) LogKeyAccess(userID, keyType, result string) {
	al.Log(AuditEvent{
		UserID:   userID,
		Action:   "key_access",
		Resource: keyType,
		Result:   result,
	})
}

// LogKeyRotation 记录密钥轮换
func (al *AuditLogger) LogKeyRotation(userID, keyType, result string) {
	al.Log(AuditEvent{
		UserID:   userID,
		Action:   "key_rotation",
		Resource: keyType,
		Result:   result,
	})
}
