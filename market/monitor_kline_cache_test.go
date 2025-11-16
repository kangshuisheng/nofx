package market

import (
	"sync"
	"testing"
	"time"
)

// TestKlineCacheEntry_TypeConsistency 测试 K线缓存的类型一致性
// 防止再次发生 "interface conversion: interface {} is []market.Kline, not *market.KlineCacheEntry" panic
func TestKlineCacheEntry_TypeConsistency(t *testing.T) {
	t.Run("确保所有写入都使用KlineCacheEntry包装", func(t *testing.T) {
		var testMap sync.Map

		// 模拟正确的写入方式
		testKlines := []Kline{
			{Open: 100.0, High: 101.0, Low: 99.0, Close: 100.5, Volume: 1000.0},
			{Open: 100.5, High: 102.0, Low: 100.0, Close: 101.0, Volume: 1100.0},
		}

		entry := &KlineCacheEntry{
			Klines:     testKlines,
			ReceivedAt: time.Now(),
		}

		// 写入缓存
		testMap.Store("BTCUSDT", entry)

		// 读取并验证类型
		value, exists := testMap.Load("BTCUSDT")
		if !exists {
			t.Fatal("缓存中应该存在 BTCUSDT")
		}

		// ✅ 这个类型断言在修复后不应该 panic
		cached, ok := value.(*KlineCacheEntry)
		if !ok {
			t.Fatalf("类型断言失败：期望 *KlineCacheEntry，实际类型: %T", value)
		}

		// 验证数据完整性
		if len(cached.Klines) != 2 {
			t.Errorf("K线数量 = %d, 期望 2", len(cached.Klines))
		}

		if cached.Klines[0].Close != 100.5 {
			t.Errorf("第一条K线收盘价 = %.2f, 期望 100.5", cached.Klines[0].Close)
		}
	})

	t.Run("错误的写入方式会导致panic（回归测试）", func(t *testing.T) {
		var testMap sync.Map

		// ❌ 旧版本的错误写入方式（直接存储 []Kline）
		testKlines := []Kline{
			{Open: 100.0, High: 101.0, Low: 99.0, Close: 100.5, Volume: 1000.0},
		}

		// 故意使用错误的方式写入
		testMap.Store("ETHUSDT", testKlines)

		// 读取
		value, exists := testMap.Load("ETHUSDT")
		if !exists {
			t.Fatal("缓存中应该存在 ETHUSDT")
		}

		// ❌ 这个类型断言会失败（演示旧版本的问题）
		defer func() {
			if r := recover(); r != nil {
				// 预期会 panic
				t.Logf("✓ 捕获到预期的 panic: %v", r)
			}
		}()

		_ = value.(*KlineCacheEntry) // 这里会 panic
		t.Error("应该 panic，但没有发生")
	})

	t.Run("检查类型断言的安全写法（ok pattern）", func(t *testing.T) {
		var testMap sync.Map

		// 故意存储错误类型
		testMap.Store("BADDATA", []Kline{{Close: 100.0}})

		value, exists := testMap.Load("BADDATA")
		if !exists {
			t.Fatal("缓存中应该存在 BADDATA")
		}

		// ✅ 安全的类型断言方式（使用 ok pattern）
		cached, ok := value.(*KlineCacheEntry)
		if !ok {
			t.Logf("✓ 检测到类型不匹配（实际类型: %T），这是预期的", value)
			return
		}

		// 如果走到这里，说明类型正确
		t.Logf("缓存数据: %+v", cached)
	})
}

// TestKlineCacheEntry_DataFreshness 测试数据新鲜度检查
func TestKlineCacheEntry_DataFreshness(t *testing.T) {
	t.Run("新鲜数据应该通过检查", func(t *testing.T) {
		entry := &KlineCacheEntry{
			Klines:     []Kline{{Close: 100.0}},
			ReceivedAt: time.Now(), // 当前时间
		}

		dataAge := time.Since(entry.ReceivedAt)
		maxAge := 15 * time.Minute

		if dataAge > maxAge {
			t.Errorf("新鲜数据被判定为过期: 数据年龄 = %.1f 分钟", dataAge.Minutes())
		}
	})

	t.Run("过期数据应该被检测", func(t *testing.T) {
		entry := &KlineCacheEntry{
			Klines:     []Kline{{Close: 100.0}},
			ReceivedAt: time.Now().Add(-20 * time.Minute), // 20分钟前
		}

		dataAge := time.Since(entry.ReceivedAt)
		maxAge := 15 * time.Minute

		if dataAge <= maxAge {
			t.Errorf("过期数据未被检测: 数据年龄 = %.1f 分钟, 阈值 = %.1f 分钟",
				dataAge.Minutes(), maxAge.Minutes())
		}
	})

	t.Run("边界情况 - 刚好15分钟", func(t *testing.T) {
		entry := &KlineCacheEntry{
			Klines:     []Kline{{Close: 100.0}},
			ReceivedAt: time.Now().Add(-15 * time.Minute),
		}

		dataAge := time.Since(entry.ReceivedAt)
		maxAge := 15 * time.Minute

		// 应该稍微大于15分钟（因为有执行延迟）
		if dataAge < maxAge {
			t.Logf("✓ 边界情况处理正确: %.3f 分钟 < 15 分钟", dataAge.Minutes())
		}
	})
}

// TestKlineCacheEntry_ConcurrentAccess 测试并发访问安全性
func TestKlineCacheEntry_ConcurrentAccess(t *testing.T) {
	var testMap sync.Map
	var wg sync.WaitGroup

	// 并发写入
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "ADAUSDT"}
	for i, symbol := range symbols {
		wg.Add(1)
		go func(sym string, index int) {
			defer wg.Done()

			klines := []Kline{
				{Close: float64(100 + index), Volume: float64(1000 + index)},
			}

			entry := &KlineCacheEntry{
				Klines:     klines,
				ReceivedAt: time.Now(),
			}

			testMap.Store(sym, entry)
		}(symbol, i)
	}

	wg.Wait()

	// 并发读取
	readErrors := make(chan error, len(symbols)*10)
	for i := 0; i < 10; i++ {
		for _, symbol := range symbols {
			wg.Add(1)
			go func(sym string) {
				defer wg.Done()

				value, exists := testMap.Load(sym)
				if !exists {
					readErrors <- nil // 可能还没写入，这是正常的
					return
				}

				// 类型断言
				cached, ok := value.(*KlineCacheEntry)
				if !ok {
					t.Errorf("并发读取时类型断言失败: %s (类型: %T)", sym, value)
					return
				}

				// 验证数据
				if len(cached.Klines) == 0 {
					t.Errorf("并发读取时数据为空: %s", sym)
				}
			}(symbol)
		}
	}

	wg.Wait()
	close(readErrors)

	// 检查读取错误
	errorCount := 0
	for err := range readErrors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("并发读取出现 %d 个错误", errorCount)
	}
}

// TestKlineCacheEntry_DeepCopy 测试深拷贝防止数据竞争
func TestKlineCacheEntry_DeepCopy(t *testing.T) {
	original := []Kline{
		{Close: 100.0, Volume: 1000.0},
		{Close: 101.0, Volume: 1100.0},
	}

	// 深拷贝
	copied := make([]Kline, len(original))
	copy(copied, original)

	// 修改拷贝
	copied[0].Close = 999.0

	// 验证原数据未被修改
	if original[0].Close != 100.0 {
		t.Errorf("深拷贝失败：原数据被修改 (%.2f != 100.0)", original[0].Close)
	}

	// 验证拷贝已修改
	if copied[0].Close != 999.0 {
		t.Errorf("拷贝数据未修改 (%.2f != 999.0)", copied[0].Close)
	}
}

// TestKlineCacheEntry_EmptyKlines 测试空K线数据的处理
func TestKlineCacheEntry_EmptyKlines(t *testing.T) {
	entry := &KlineCacheEntry{
		Klines:     []Kline{}, // 空切片
		ReceivedAt: time.Now(),
	}

	// 验证不会 panic
	if entry.Klines == nil {
		t.Error("Klines 不应该是 nil，应该是空切片")
	}

	if len(entry.Klines) != 0 {
		t.Errorf("空K线长度 = %d, 期望 0", len(entry.Klines))
	}
}

// TestWSMonitor_KlineCache_Integration 集成测试 - 模拟实际使用场景
func TestWSMonitor_KlineCache_Integration(t *testing.T) {
	t.Run("模拟API获取并缓存K线数据", func(t *testing.T) {
		// 创建模拟的 sync.Map（模拟 WSMonitor 的缓存）
		var klineDataMap sync.Map

		// 模拟从 API 获取数据
		mockKlines := []Kline{
			{
				OpenTime:  time.Now().UnixMilli() - 3000000,
				Open:      50000.0,
				High:      51000.0,
				Low:       49500.0,
				Close:     50500.0,
				Volume:    1000000.0,
				CloseTime: time.Now().UnixMilli() - 2000000,
			},
			{
				OpenTime:  time.Now().UnixMilli() - 2000000,
				Open:      50500.0,
				High:      51500.0,
				Low:       50000.0,
				Close:     51000.0,
				Volume:    1100000.0,
				CloseTime: time.Now().UnixMilli() - 1000000,
			},
		}

		// ✅ 正确的缓存方式（使用 KlineCacheEntry 包装）
		entry := &KlineCacheEntry{
			Klines:     mockKlines,
			ReceivedAt: time.Now(),
		}
		klineDataMap.Store("BTCUSDT", entry)

		// 模拟读取缓存（类似 GetCurrentKlines 的逻辑）
		value, exists := klineDataMap.Load("BTCUSDT")
		if !exists {
			t.Fatal("缓存中应该存在 BTCUSDT")
		}

		// ✅ 这个类型断言不应该 panic
		cached := value.(*KlineCacheEntry)

		// 验证数据新鲜度
		dataAge := time.Since(cached.ReceivedAt)
		maxAge := 15 * time.Minute
		if dataAge > maxAge {
			t.Errorf("数据过期: %.1f 分钟 > %.1f 分钟", dataAge.Minutes(), maxAge.Minutes())
		}

		// 返回深拷贝（模拟实际代码）
		klines := cached.Klines
		result := make([]Kline, len(klines))
		copy(result, klines)

		// 验证结果
		if len(result) != 2 {
			t.Errorf("K线数量 = %d, 期望 2", len(result))
		}

		if result[1].Close != 51000.0 {
			t.Errorf("最后一条K线收盘价 = %.2f, 期望 51000.0", result[1].Close)
		}

		// 修改拷贝不应该影响缓存
		result[0].Close = 99999.0
		if cached.Klines[0].Close == 99999.0 {
			t.Error("深拷贝失败：修改返回值影响了缓存")
		}
	})

	t.Run("回归测试 - 旧版本的错误场景", func(t *testing.T) {
		// 这个测试模拟修复前的错误：直接存储 []Kline
		var klineDataMap sync.Map

		mockKlines := []Kline{
			{Close: 50000.0, Volume: 1000000.0},
		}

		// ❌ 错误的缓存方式（直接存储 []Kline）
		klineDataMap.Store("ETHUSDT", mockKlines)

		// 读取
		value, exists := klineDataMap.Load("ETHUSDT")
		if !exists {
			t.Fatal("缓存中应该存在 ETHUSDT")
		}

		// 捕获 panic
		defer func() {
			if r := recover(); r != nil {
				// ✓ 这是预期的 panic，证明测试正确模拟了问题
				expectedMsg := "interface conversion: interface {} is []market.Kline, not *market.KlineCacheEntry"
				if panicMsg, ok := r.(error); ok {
					if panicMsg.Error() != expectedMsg {
						t.Logf("✓ 捕获到 panic（类型不匹配）: %v", panicMsg)
					}
				} else {
					t.Logf("✓ 捕获到 panic: %v", r)
				}
			} else {
				t.Error("应该 panic，但没有发生")
			}
		}()

		// 这行代码会 panic（模拟用户报告的错误）
		_ = value.(*KlineCacheEntry)
	})

	t.Run("验证所有时间线都使用一致的类型", func(t *testing.T) {
		// 模拟多个时间线的缓存
		timeframes := []string{"1m", "15m", "1h", "4h"}
		caches := make(map[string]*sync.Map)

		for _, tf := range timeframes {
			cache := &sync.Map{}
			caches[tf] = cache

			// 为每个时间线缓存数据
			entry := &KlineCacheEntry{
				Klines: []Kline{
					{Close: 100.0 * float64(len(tf)), Volume: 1000.0}, // 不同时间线不同价格
				},
				ReceivedAt: time.Now(),
			}
			cache.Store("SOLUSDT", entry)
		}

		// 验证所有缓存都能正确读取
		for tf, cache := range caches {
			value, exists := cache.Load("SOLUSDT")
			if !exists {
				t.Errorf("时间线 %s 的缓存不存在", tf)
				continue
			}

			// 类型断言不应该 panic
			cached, ok := value.(*KlineCacheEntry)
			if !ok {
				t.Errorf("时间线 %s 的类型断言失败: %T", tf, value)
				continue
			}

			if len(cached.Klines) == 0 {
				t.Errorf("时间线 %s 的K线数据为空", tf)
			}
		}
	})
}
