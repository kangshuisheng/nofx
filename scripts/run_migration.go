//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("用法: go run run_migration.go <db_path> <migration_sql_file>")
	}

	dbPath := os.Args[1]
	sqlFile := os.Args[2]

	// 备份数据库
	backupPath := dbPath + ".backup_" + time.Now().Format("20060102_150405")
	log.Printf("📦 创建备份: %s", backupPath)

	input, err := ioutil.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("读取数据库失败: %v", err)
	}

	err = ioutil.WriteFile(backupPath, input, 0600)
	if err != nil {
		log.Fatalf("创建备份失败: %v", err)
	}

	log.Printf("✅ 备份完成")

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 验证数据库完整性
	log.Printf("🔍 验证数据库完整性...")
	var result string
	err = db.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil {
		log.Fatalf("完整性检查失败: %v", err)
	}

	if result != "ok" {
		log.Fatalf("❌ 数据库完整性检查失败: %s", result)
	}
	log.Printf("✅ 数据库完整性正常")

	// 读取 SQL 文件
	sqlContent, err := ioutil.ReadFile(sqlFile)
	if err != nil {
		log.Fatalf("读取 SQL 文件失败: %v", err)
	}

	// 执行 SQL
	log.Printf("🔄 执行迁移: %s", sqlFile)
	_, err = db.Exec(string(sqlContent))
	if err != nil {
		log.Printf("❌ 执行迁移失败: %v", err)

		// 自动回滚
		log.Printf("🔙 正在回滚...")
		backup, _ := ioutil.ReadFile(backupPath)
		ioutil.WriteFile(dbPath, backup, 0600)
		log.Fatal("已回滚到备份版本")
	}

	log.Printf("✅ 迁移成功完成")
	log.Printf("💡 备份文件保存在: %s", backupPath)

	// 验证索引创建
	log.Printf("\n📊 验证索引列表:")
	rows, err := db.Query(`
		SELECT name, tbl_name
		FROM sqlite_master
		WHERE type = 'index' AND name LIKE 'idx_%'
		ORDER BY tbl_name, name
	`)
	if err != nil {
		log.Printf("⚠️ 查询索引失败: %v", err)
		return
	}
	defer rows.Close()

	indexCount := 0
	for rows.Next() {
		var name, tblName string
		rows.Scan(&name, &tblName)
		log.Printf("  ✓ %s.%s", tblName, name)
		indexCount++
	}

	log.Printf("\n✅ 共创建 %d 个索引", indexCount)

	// 性能测试
	log.Printf("\n⏱️  性能测试:")
	testQueries := []struct {
		name  string
		query string
	}{
		{"用户 AI 模型查询", "SELECT * FROM ai_models WHERE user_id = 'test' LIMIT 1"},
		{"用户交易所查询", "SELECT * FROM exchanges WHERE user_id = 'test' LIMIT 1"},
		{"用户 Trader 查询", "SELECT * FROM traders WHERE user_id = 'test' LIMIT 1"},
		{"运行中 Trader", "SELECT COUNT(*) FROM traders WHERE is_running = 1"},
	}

	for _, test := range testQueries {
		start := time.Now()
		_, _ = db.Exec(test.query)
		duration := time.Since(start)
		log.Printf("  %s: %v", test.name, duration)
	}

	fmt.Println("\n🎉 迁移完成!")
}
