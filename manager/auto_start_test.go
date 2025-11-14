package manager

import (
	"testing"
)

// TestStartRunningTraders_BasicFunctionality 测试基本功能
// TODO: 完整的端到端测试需要在 Day 8 补充（需要 Database API 完整 mock）
//
// 与 trader_manager_test.go:549-570 中的测试类似，Database API 在多配置支持后
// 发生了重大变更，需要完整的 mock 才能进行端到端测试。
//
// 所需的完整测试覆盖（Day 8 补充）：
// 1. 测试单用户场景下自动启动标记为运行状态的 traders
// 2. 测试多用户场景下自动启动各自的 traders
// 3. 测试没有运行中的 traders 时的行为（应返回 nil）
// 4. 测试数据库错误处理（GetAllUsers 失败、GetTraders 失败等）
// 5. 测试 trader 未加载到内存时的跳过逻辑
//
// 参考实现：
// - StartRunningTraders() 在 manager/trader_manager.go:444-489
// - 核心逻辑：遍历所有用户 → 获取 IsRunning=true 的 traders → 启动对应的 AutoTrader
//
// 当前状态：
// - ✅ 功能代码已整合并在 main.go:393-397 使用
// - ⚠️  测试待补充（与其他 Manager 测试一样需要 Database API mock）
func TestStartRunningTraders_BasicFunctionality(t *testing.T) {
	// 标记为跳过，等待 Day 8 补充完整测试
	t.Skip("TODO (Day 8): Requires full Database API mock - see trader_manager_test.go:549 for similar examples")
}

// TestStartRunningTraders_Integration 集成测试
// TODO (Day 8): 使用真实数据库的集成测试
func TestStartRunningTraders_Integration(t *testing.T) {
	t.Skip("TODO (Day 8): Integration test with real Database API")
}

// TestStartRunningTraders_ErrorHandling 错误处理测试
// TODO (Day 8): 测试各种错误场景
func TestStartRunningTraders_ErrorHandling(t *testing.T) {
	t.Skip("TODO (Day 8): Test error scenarios (DB failures, invalid traders, etc.)")
}
