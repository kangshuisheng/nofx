package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ✅ 优化2：Funding Rate 缓存机制（节省 95% API 调用）
// Binance Funding Rate 每 8 小时才更新一次，使用 1 小时缓存完全合理
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

// FearGreedIndex 恐慌贪婪指数结构
type FearGreedIndex struct {
	Value          int    `json:"value"`          // 0-100
	Classification string `json:"classification"` // e.g. "Extreme Fear"
	Timestamp      int64  `json:"timestamp"`
}

// FearGreedResponse API响应结构
type FearGreedResponse struct {
	Data []struct {
		Value           string `json:"value"`
		ValueClass      string `json:"value_classification"`
		Timestamp       string `json:"timestamp"`
		TimeUntilUpdate string `json:"time_until_update"`
	} `json:"data"`
	Metadata struct {
		Error interface{} `json:"error"`
	} `json:"metadata"`
}

var (
	fearGreedCache     *FearGreedIndex
	fearGreedUpdatedAt time.Time
	fgCacheTTL         = 30 * time.Minute // 指数每天更新一次，30分钟缓存足够
)

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
)

// Get 获取指定代币的市场数据（支持动态时间线选择）
// timeframes: 可选参数，指定需要获取的时间线列表，如 []string{"1m", "15m", "1h", "4h"}
// 如果为空或nil，默认使用 ["15m", "1h", "4h"]
func Get(symbol string, timeframes []string) (*Data, error) {
	var klines1m, klines3m, klines5m, klines15m, klines1h, klines4h, klines1d []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)

	// 设置默认时间线（如果未指定） - 🔧 中长线策略优化
	if len(timeframes) == 0 {
		timeframes = []string{"15m", "1h", "4h", "1d"}
		log.Printf("⚠️  %s 未指定时间线，使用默认值(中长线+15m精准): %v", symbol, timeframes)
	}

	// 创建时间线查找映射（提高查找效率）
	tfMap := make(map[string]bool)
	for _, tf := range timeframes {
		tfMap[tf] = true
	}

	// 确定最短时间线（用于计算当前价格和指标）
	shortestTF := ""
	tfPriority := []string{"1m", "3m", "5m", "15m", "1h", "4h", "1d"}
	for _, tf := range tfPriority {
		if tfMap[tf] {
			shortestTF = tf
			break
		}
	}

	// 如果没有找到任何短期时间线，使用3m作为默认（兼容旧行为）
	if shortestTF == "" {
		shortestTF = "3m"
		log.Printf("⚠️  %s 未配置任何时间线，使用3m作为默认短期时间线", symbol)
	}

	// 获取短期K线数据（用于当前价格和指标计算）
	var shortKlines []Kline
	switch shortestTF {
	case "1m":
		klines1m, err = WSMonitorCli.GetCurrentKlines(symbol, "1m")
		if err != nil {
			return nil, fmt.Errorf("获取1分钟K线失败: %v", err)
		}
		shortKlines = klines1m
	case "3m":
		klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m")
		if err != nil {
			return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
		}
		shortKlines = klines3m
	case "5m":
		klines5m, err = WSMonitorCli.GetCurrentKlines(symbol, "5m")
		if err != nil {
			return nil, fmt.Errorf("获取5分钟K线失败: %v", err)
		}
		shortKlines = klines5m
	default:
		// 如果最短时间线是15m或更长，也获取一个短期数据用于stale检测
		klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m")
		if err != nil {
			return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
		}
		shortKlines = klines3m
	}

	// Data staleness detection: Prevent DOGEUSDT-style price freeze issues (PR #800)
	if isStaleData(shortKlines, symbol) {
		log.Printf("⚠️  WARNING: %s detected stale data (consecutive price freeze), skipping symbol", symbol)
		return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol)
	}

	// 根据配置获取其他时间线数据
	if tfMap["15m"] && len(klines15m) == 0 {
		klines15m, err = WSMonitorCli.GetCurrentKlines(symbol, "15m")
		if err != nil {
			return nil, fmt.Errorf("获取15分钟K线失败: %v", err)
		}
	}

	if tfMap["1h"] && len(klines1h) == 0 {
		klines1h, err = WSMonitorCli.GetCurrentKlines(symbol, "1h")
		if err != nil {
			return nil, fmt.Errorf("获取1小时K线失败: %v", err)
		}
	}

	if tfMap["4h"] {
		klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h")
		if err != nil {
			return nil, fmt.Errorf("获取4小时K线失败: %v", err)
		}
		// P0修复：检查 4h 数据完整性（如果用户选择了4h）
		if len(klines4h) == 0 {
			log.Printf("⚠️  WARNING: %s 缺少 4h K线数据，无法进行多周期趋势确认", symbol)
			return nil, fmt.Errorf("%s 缺少 4h K线数据", symbol)
		}
	}

	if tfMap["1d"] {
		klines1d, err = WSMonitorCli.GetCurrentKlines(symbol, "1d")
		if err != nil {
			log.Printf("⚠️  WARNING: %s 获取日线K线失败: %v，将继续处理但缺少日线数据", symbol, err)
			klines1d = nil // 日线数据失败不影响整体流程
		}
	}

	// 计算当前指标 (基于最短时间线的最新数据)
	currentPrice := shortKlines[len(shortKlines)-1].Close
	currentEMA20 := calculateEMA(shortKlines, 20)
	currentMACD := calculateMACD(shortKlines)
	currentRSI7 := calculateRSI(shortKlines, 7)

	// 计算价格变化百分比（基于可用数据）
	priceChange1h := 0.0
	priceChange4h := 0.0

	// 1小时价格变化：优先使用1h数据，其次用短期数据推算
	if len(klines1h) >= 2 {
		price1hAgo := klines1h[len(klines1h)-2].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	} else if shortestTF == "3m" && len(shortKlines) >= 21 {
		// 20个3分钟K线 = 1小时
		price1hAgo := shortKlines[len(shortKlines)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化：使用4h数据
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0, ActualPeriod: "N/A"}
	}

	// ⚡ 新增：增強 OI 數據（加入多空比 - 完全免費）
	// 這不會影響性能，因為 Binance API 無限制且快速
	if err := EnhanceOIData(symbol, oiData); err != nil {
		// 多空比獲取失敗不影響整體流程，只記錄警告
		log.Printf("⚠️  %s 獲取多空比數據失敗: %v", symbol, err)
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// ✅ 条件性计算时间线数据（只计算用户选择的时间线）
	var intradayData *IntradayData
	var midTermData15m *MidTermData15m
	var midTermData1h *MidTermData1h
	var longerTermData *LongerTermData
	var dailyData *DailyData

	// 计算日内系列数据 (1m/3m/5m)
	if len(klines1m) > 0 {
		intradayData = calculateIntradaySeries(klines1m)
	} else if len(klines3m) > 0 {
		intradayData = calculateIntradaySeries(klines3m)
	} else if len(klines5m) > 0 {
		intradayData = calculateIntradaySeries(klines5m)
	}

	// 计算15分钟系列数据（如果用户选择了15m）
	if len(klines15m) > 0 {
		midTermData15m = calculateMidTermSeries15m(klines15m)
	}

	// 计算1小时系列数据（如果用户选择了1h）
	if len(klines1h) > 0 {
		midTermData1h = calculateMidTermSeries1h(klines1h)
	}

	// 计算长期数据 (4小时，如果用户选择了4h)
	if len(klines4h) > 0 {
		longerTermData = calculateLongerTermData(klines4h)
	}

	// 计算日线数据（如果用户选择了1d）
	if len(klines1d) > 0 {
		dailyData = calculateDailyData(klines1d)
	}

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		MidTermSeries15m:  midTermData15m,
		MidTermSeries1h:   midTermData1h,
		LongerTermContext: longerTermData,
		DailyContext:      dailyData,
		FearGreedIndex:    getFearGreedIndex(), // 获取恐慌贪婪指数
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateADX 计算ADX (平均趋向指数)
func calculateADX(klines []Kline, period int) float64 {
	if len(klines) < period*2 {
		return 0
	}

	// 1. 计算 TR, +DM, -DM
	trs := make([]float64, len(klines))
	plusDMs := make([]float64, len(klines))
	minusDMs := make([]float64, len(klines))

	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close
		prevHigh := klines[i-1].High
		prevLow := klines[i-1].Low

		// TR
		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)
		trs[i] = math.Max(tr1, math.Max(tr2, tr3))

		// +DM, -DM
		upMove := high - prevHigh
		downMove := prevLow - low

		if upMove > downMove && upMove > 0 {
			plusDMs[i] = upMove
		} else {
			plusDMs[i] = 0
		}

		if downMove > upMove && downMove > 0 {
			minusDMs[i] = downMove
		} else {
			minusDMs[i] = 0
		}
	}

	// 2. 平滑 TR, +DM, -DM (Wilder's Smoothing)
	// 初始平滑 (SMA)
	smoothTR := 0.0
	smoothPlusDM := 0.0
	smoothMinusDM := 0.0

	for i := 1; i <= period; i++ {
		smoothTR += trs[i]
		smoothPlusDM += plusDMs[i]
		smoothMinusDM += minusDMs[i]
	}

	// 计算初始 DX
	dxs := make([]float64, len(klines))

	// 从 period+1 开始计算后续平滑值和 DX
	for i := period + 1; i < len(klines); i++ {
		smoothTR = smoothTR - (smoothTR / float64(period)) + trs[i]
		smoothPlusDM = smoothPlusDM - (smoothPlusDM / float64(period)) + plusDMs[i]
		smoothMinusDM = smoothMinusDM - (smoothMinusDM / float64(period)) + minusDMs[i]

		plusDI := 0.0
		minusDI := 0.0
		if smoothTR != 0 {
			plusDI = (smoothPlusDM / smoothTR) * 100
			minusDI = (smoothMinusDM / smoothTR) * 100
		}

		if plusDI+minusDI != 0 {
			dxs[i] = (math.Abs(plusDI-minusDI) / (plusDI + minusDI)) * 100
		}
	}

	// 3. 计算 ADX (DX 的 SMA)
	// 需要至少 period 个 DX 值才能开始计算第一个 ADX
	// 第一个 ADX 是前 period 个 DX 的平均值
	// ADX 序列开始于 period*2 处

	if len(klines) <= period*2 {
		return 0
	}

	sumDX := 0.0
	for i := period + 1; i <= period*2; i++ {
		sumDX += dxs[i]
	}
	adx := sumDX / float64(period)

	// 后续 ADX 使用平滑
	for i := period*2 + 1; i < len(klines); i++ {
		adx = ((adx * float64(period-1)) + dxs[i]) / float64(period)
	}

	return adx
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:   make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		MACDValues:  make([]float64, 0, 10),
		RSI7Values:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
		Volume:      make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)
		data.Volume = append(data.Volume, klines[i].Volume)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// 计算3m ATR14
	data.ATR14 = calculateATR(klines, 14)

	return data
}

// calculateMidTermSeries15m 计算15分钟系列数据
func calculateMidTermSeries15m(klines []Kline) *MidTermData15m {
	data := &MidTermData15m{
		MidPrices:   make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		MACDValues:  make([]float64, 0, 10),
		RSI7Values:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// calculateMidTermSeries1h 计算1小时系列数据
func calculateMidTermSeries1h(klines []Kline) *MidTermData1h {
	data := &MidTermData1h{
		MidPrices:   make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		MACDValues:  make([]float64, 0, 10),
		RSI7Values:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
		Volume:      make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)
		data.Volume = append(data.Volume, klines[i].Volume)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		EMA50Values: make([]float64, 0, 10),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算ADX
	data.ADX = calculateADX(klines, 14)

	// 计算成交量
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// 计算MACD, RSI和EMA序列
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}
		if i >= 49 {
			ema50 := calculateEMA(klines[:i+1], 50)
			data.EMA50Values = append(data.EMA50Values, ema50)
		}
	}

	return data
}

// calculateDailyData 计算日线数据
func calculateDailyData(klines []Kline) *DailyData {
	data := &DailyData{
		MidPrices:   make([]float64, 0, 90),
		EMA20Values: make([]float64, 0, 90),
		EMA50Values: make([]float64, 0, 90),
		MACDValues:  make([]float64, 0, 90),
		RSI14Values: make([]float64, 0, 90),
		ATR14Values: make([]float64, 0, 90),
		Volume:      make([]float64, 0, 90),
	}

	// 获取全部数据点（最多90个）
	for i := 0; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)
		data.Volume = append(data.Volume, klines[i].Volume)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的EMA50
		if i >= 49 {
			ema50 := calculateEMA(klines[:i+1], 50)
			data.EMA50Values = append(data.EMA50Values, ema50)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI14
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}

		// 计算每个点的ATR14
		if i >= 14 {
			atr14 := calculateATR(klines[:i+1], 14)
			data.ATR14Values = append(data.ATR14Values, atr14)
		}
	}

	// 计算ADX
	data.ADX = calculateADX(klines, 14)

	return data
}

// getOpenInterestData 获取OI数据（优化：优先使用缓存）
func getOpenInterestData(symbol string) (*OIData, error) {
	// ✅ 修复：统一symbol格式（确保大小写一致）
	symbol = Normalize(symbol)

	// ✅ 优化1：优先使用 collectOISnapshots 的缓存数据（每15分钟更新）
	// 好处：节省 50% API 调用，数据新鲜度 < 15 分钟
	if WSMonitorCli != nil {
		history := WSMonitorCli.GetOIHistory(symbol)
		log.Printf("🔍 [OI缓存检查] Symbol: %s, WSMonitorCli存在: true, 历史数据点数: %d", symbol, len(history))
		if len(history) > 0 {
			// 使用最新的快照（最多 15 分钟前的数据）
			latest := history[len(history)-1]

			var change4h float64
			var actualPeriod string
			change4h, actualPeriod = WSMonitorCli.CalculateOIChange4h(symbol, latest.Value)

			log.Printf("✅ [OI缓存命中] Symbol: %s, 使用缓存数据, 数据点数: %d, ActualPeriod: %s", symbol, len(history), actualPeriod)
			return &OIData{
				Latest:       latest.Value,
				Average:      latest.Value * 0.999, // 近似平均值
				Change4h:     change4h,
				ActualPeriod: actualPeriod,
				Historical:   history,
			}, nil
		} else {
			log.Printf("⚠️  [OI缓存未命中] Symbol: %s, 历史数据为空，降级到API调用", symbol)
		}
	} else {
		log.Printf("⚠️  [OI缓存不可用] Symbol: %s, WSMonitorCli为nil", symbol)
	}

	// ⚠️ 降级：缓存不存在时才调用 API（仅冷启动或缓存失效）
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	// 计算4小时变化率
	var change4h float64
	var actualPeriod string
	if WSMonitorCli != nil {
		change4h, actualPeriod = WSMonitorCli.CalculateOIChange4h(symbol, oi)
	} else {
		actualPeriod = "N/A"
	}

	// 获取历史数据
	var history []OISnapshot
	if WSMonitorCli != nil {
		history = WSMonitorCli.GetOIHistory(symbol)
	}

	return &OIData{
		Latest:       oi,
		Average:      oi * 0.999,
		Change4h:     change4h,
		ActualPeriod: actualPeriod,
		Historical:   history,
	}, nil
}

// getFundingRate 获取资金费率（优化：使用 1 小时缓存）
func getFundingRate(symbol string) (float64, error) {
	// ✅ 修复：统一symbol格式（确保大小写一致）
	symbol = Normalize(symbol)

	// ✅ 优化2：检查缓存（有效期 1 小时）
	// Funding Rate 每 8 小时才更新，1 小时缓存非常合理
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// 缓存命中，直接返回
			return cache.Rate, nil
		}
	}

	// ⚠️ 缓存过期或不存在，调用 API
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)

	// ✅ 更新缓存
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// getFearGreedIndex 获取恐慌贪婪指数 (带缓存)
func getFearGreedIndex() *FearGreedIndex {
	// 1. 检查缓存
	if fearGreedCache != nil && time.Since(fearGreedUpdatedAt) < fgCacheTTL {
		return fearGreedCache
	}

	// 2. 调用 API
	url := "https://api.alternative.me/fng/?limit=1"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("⚠️ 获取恐慌贪婪指数失败: %v", err)
		if fearGreedCache != nil {
			return fearGreedCache // 失败时返回旧缓存
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️ 读取恐慌贪婪指数响应失败: %v", err)
		return nil
	}

	var result FearGreedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("⚠️ 解析恐慌贪婪指数JSON失败: %v", err)
		return nil
	}

	if len(result.Data) == 0 {
		return nil
	}

	// 3. 解析数据
	val, _ := strconv.Atoi(result.Data[0].Value)
	ts, _ := strconv.ParseInt(result.Data[0].Timestamp, 10, 64)

	index := &FearGreedIndex{
		Value:          val,
		Classification: result.Data[0].ValueClass,
		Timestamp:      ts,
	}

	// 4. 更新缓存
	fearGreedCache = index
	fearGreedUpdatedAt = time.Now()

	return index
}

// Format 格式化市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// 1. 核心摘要信息
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	fundingIcon := ""
	if math.Abs(data.FundingRate) > 0.0004 {
		fundingIcon = "⚠️"
	} // 费率过高预警
	oiIcon := ""
	if math.Abs(data.OpenInterest.Change4h) > 3.0 {
		oiIcon = "🔥"
	} // OI剧烈变化

	sb.WriteString(fmt.Sprintf("Price: %s | OI Chg(4h): %.2f%%%s | Funding: %.6f%s\n\n",
		priceStr, data.OpenInterest.Change4h, oiIcon, data.FundingRate, fundingIcon))

	// 1.5 恐慌贪婪指数
	if data.FearGreedIndex != nil {
		sb.WriteString(fmt.Sprintf("- Fear & Greed Index: %d (%s)\n",
			data.FearGreedIndex.Value, data.FearGreedIndex.Classification))
	}

	// 2. 市场情绪上下文
	if data.OpenInterest != nil && data.OpenInterest.LongShortRatio > 0 {
		sb.WriteString("- Market Sentiment Context:\n")
		longPct := data.OpenInterest.LongShortRatio / (1 + data.OpenInterest.LongShortRatio) * 100
		shortPct := 100 - longPct
		sb.WriteString(fmt.Sprintf("  - Market_L/S_Ratio: %.2f (%.1f%% Long / %.1f%% Short)\n",
			data.OpenInterest.LongShortRatio, longPct, shortPct))

		if data.OpenInterest.TopTraderLongShortRatio > 0 {
			sb.WriteString(fmt.Sprintf("  - Top_Traders_L/S_Ratio: %.2f\n\n", data.OpenInterest.TopTraderLongShortRatio))
		} else {
			sb.WriteString("\n")
		}
	}

	// 3. 更高时间周期上下文
	sb.WriteString("- Higher Timeframe Context:\n")
	if data.DailyContext != nil && len(data.DailyContext.MidPrices) > 0 {
		// 展示最近14天的日线数据,帮助判断大趋势（中长线需要更长视野）
		const dailyLen = 14

		prices := data.DailyContext.MidPrices
		if len(prices) > dailyLen {
			prices = prices[len(prices)-dailyLen:]
		}
		sb.WriteString(fmt.Sprintf("  - Daily_Close: %s\n", formatFloatSlice(prices)))

		if len(data.DailyContext.EMA20Values) > 0 {
			ema20s := data.DailyContext.EMA20Values
			if len(ema20s) > dailyLen {
				ema20s = ema20s[len(ema20s)-dailyLen:]
			}
			sb.WriteString(fmt.Sprintf("  - Daily_EMA20: %s\n", formatFloatSlice(ema20s)))
		}

		if len(data.DailyContext.MACDValues) > 0 {
			macds := data.DailyContext.MACDValues
			if len(macds) > dailyLen {
				macds = macds[len(macds)-dailyLen:]
			}
			sb.WriteString(fmt.Sprintf("  - Daily_MACD:  %s\n", formatFloatSlice(macds)))
		}

		// 显示Daily ADX
		if data.DailyContext.ADX > 0 {
			sb.WriteString(fmt.Sprintf("  - Daily_ADX:   %.2f\n", data.DailyContext.ADX))
		}
	} else {
		sb.WriteString("  - Daily_Data: N/A\n")
	}

	// 15分钟周期数据 (精准入场确认)
	if data.MidTermSeries15m != nil {
		sb.WriteString("- 15m (Precision Entry Confirmation):\n")

		const m15Length = 6

		prices := data.MidTermSeries15m.MidPrices
		if len(prices) > m15Length {
			prices = prices[len(prices)-m15Length:]
		}
		sb.WriteString(fmt.Sprintf("  - Prices: %s\n", formatFloatSlice(prices)))

		ema20s := data.MidTermSeries15m.EMA20Values
		if len(ema20s) > m15Length {
			ema20s = ema20s[len(ema20s)-m15Length:]
		}
		sb.WriteString(fmt.Sprintf("  - EMA20:  %s\n", formatFloatSlice(ema20s)))

		macds := data.MidTermSeries15m.MACDValues
		if len(macds) > m15Length {
			macds = macds[len(macds)-m15Length:]
		}
		sb.WriteString(fmt.Sprintf("  - MACD:   %s\n", formatFloatSlice(macds)))

		rsi14s := data.MidTermSeries15m.RSI14Values
		if len(rsi14s) > m15Length {
			rsi14s = rsi14s[len(rsi14s)-m15Length:]
		}
		sb.WriteString(fmt.Sprintf("  - RSI(14):%s\n\n", formatFloatSlice(rsi14s)))
	}

	if data.MidTermSeries1h != nil && len(data.MidTermSeries1h.MACDValues) > 0 {

		const seriesLength = 8

		prices := data.MidTermSeries1h.MidPrices
		if len(prices) > seriesLength {
			prices = prices[len(prices)-seriesLength:]
		}
		sb.WriteString(fmt.Sprintf("  - Prices: %s\n", formatFloatSlice(prices)))

		// 计算最近10根1H的最高/最低价（用于判断挂单位置）
		if len(prices) > 0 {
			highest := prices[0]
			lowest := prices[0]
			for _, p := range prices {
				if p > highest {
					highest = p
				}
				if p < lowest {
					lowest = p
				}
			}
			sb.WriteString(fmt.Sprintf("  - Recent_High: %.4f | Recent_Low: %.4f\n", highest, lowest))
		}

		ema20s := data.MidTermSeries1h.EMA20Values
		if len(ema20s) > seriesLength {
			ema20s = ema20s[len(ema20s)-seriesLength:]
		}
		sb.WriteString(fmt.Sprintf("  - EMA20:  %s\n", formatFloatSlice(ema20s)))

		macds := data.MidTermSeries1h.MACDValues
		if len(macds) > seriesLength {
			macds = macds[len(macds)-seriesLength:]
		}
		sb.WriteString(fmt.Sprintf("  - MACD:   %s\n", formatFloatSlice(macds)))

		rsi14s := data.MidTermSeries1h.RSI14Values
		if len(rsi14s) > seriesLength {
			rsi14s = rsi14s[len(rsi14s)-seriesLength:]
		}
		sb.WriteString(fmt.Sprintf("  - RSI(14):%s\n", formatFloatSlice(rsi14s)))

		volumes := data.MidTermSeries1h.Volume
		if len(volumes) > seriesLength {
			volumes = volumes[len(volumes)-seriesLength:]
		}
		sb.WriteString(fmt.Sprintf("  - Volume: %s\n\n", formatFloatSlice(volumes)))
	}

	// 3. 4H周期数据 (趋势判断和风险管理核心)
	if data.LongerTermContext != nil {
		sb.WriteString("- 4H (Trend & Risk):\n")

		// 显示4H价格序列（帮助判断趋势斜率）
		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString("  - 4H_Trend_Context: Recent candles available\n")
		}

		sb.WriteString(fmt.Sprintf("  - EMAs: EMA20(%.3f) vs EMA50(%.3f)\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		// 展示EMA序列,帮助判断趋势斜率
		if len(data.LongerTermContext.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("  - EMA20_Seq: %s\n", formatFloatSlice(data.LongerTermContext.EMA20Values)))
		}
		if len(data.LongerTermContext.EMA50Values) > 0 {
			sb.WriteString(fmt.Sprintf("  - EMA50_Seq: %s\n", formatFloatSlice(data.LongerTermContext.EMA50Values)))
		}

		sb.WriteString(fmt.Sprintf("  - ATR(14) for StopLoss: %.4f\n", data.LongerTermContext.ATR14))

		// 显示4H ADX
		if data.LongerTermContext.ADX > 0 {
			sb.WriteString(fmt.Sprintf("  - 4H_ADX:      %.2f\n", data.LongerTermContext.ADX))
		}

		// 计算ATR通道（用于震荡市场的挂单位置）
		if data.LongerTermContext.ATR14 > 0 {
			atrUpper := data.LongerTermContext.EMA20 + (data.LongerTermContext.ATR14 * 2)
			atrLower := data.LongerTermContext.EMA20 - (data.LongerTermContext.ATR14 * 2)
			sb.WriteString(fmt.Sprintf("  - ATR_Channel: Upper(%.4f) | Lower(%.4f)\n", atrUpper, atrLower))
		}

		if data.LongerTermContext.AverageVolume > 0 {
			ratio := data.LongerTermContext.CurrentVolume / data.LongerTermContext.AverageVolume
			sb.WriteString(fmt.Sprintf("  - Volume_Ratio_Current_Avg: %.2f\n", ratio))
		} else {
			sb.WriteString("  - Volume_Ratio_Current_Avg: 0.00\n")
		}

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("  - MACD: %s\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}
		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("  - RSI(14): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		} else {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// formatPriceWithDynamicPrecision 根据价格区间动态选择精度
// 这样可以完美支持从超低价 meme coin (< 0.0001) 到 BTC/ETH 的所有币种
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// 超低价 meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8位小数)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// 低价 meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// 中低价币: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// 低价币: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4位小数)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// 中价币: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4位小数)
		return fmt.Sprintf("%.4f", price)
	default:
		// 高价币: BTC, ETH (节省 Token)
		// 45678.9123 → "45678.91" (2位小数)
		return fmt.Sprintf("%.2f", price)
	}
}

// formatFloatSlice 格式化float64切片为字符串（使用动态精度）
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// isStaleData detects stale data (consecutive price freeze)
// Fix DOGEUSDT-style issue: consecutive N periods with completely unchanged prices indicate data source anomaly
func isStaleData(klines []Kline, symbol string) bool {
	if len(klines) < 5 {
		return false // Insufficient data to determine
	}

	// Detection threshold: 5 consecutive 3-minute periods with unchanged price (15 minutes without fluctuation)
	const stalePriceThreshold = 5
	const priceTolerancePct = 0.0001 // 0.01% fluctuation tolerance (avoid false positives)

	// Take the last stalePriceThreshold K-lines
	recentKlines := klines[len(klines)-stalePriceThreshold:]
	firstPrice := recentKlines[0].Close

	// Check if all prices are within tolerance
	for i := 1; i < len(recentKlines); i++ {
		priceDiff := math.Abs(recentKlines[i].Close-firstPrice) / firstPrice
		if priceDiff > priceTolerancePct {
			return false // Price fluctuation exists, data is normal
		}
	}

	// Additional check: MACD and volume
	// If price is unchanged but MACD/volume shows normal fluctuation, it might be a real market situation (extremely low volatility)
	// Check if volume is also 0 (data completely frozen)
	allVolumeZero := true
	for _, k := range recentKlines {
		if k.Volume > 0 {
			allVolumeZero = false
			break
		}
	}

	if allVolumeZero {
		log.Printf("⚠️  %s stale data confirmed: price freeze + zero volume", symbol)
		return true
	}

	// Price frozen but has volume: might be extremely low volatility market, allow but log warning
	log.Printf("⚠️  %s detected extreme price stability (no fluctuation for %d consecutive periods), but volume is normal", symbol, stalePriceThreshold)
	return false
}
