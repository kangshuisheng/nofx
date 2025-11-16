package market

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidDataSource 封装 Hyperliquid 作为数据源
type HyperliquidDataSource struct {
	info *hyperliquid.Info
	ctx  context.Context
	name string
}

// NewHyperliquidDataSource 创建 Hyperliquid 数据源实例（不需要认证，只用于获取公开市场数据）
func NewHyperliquidDataSource(testnet bool) *HyperliquidDataSource {
	// 选择 API URL
	baseURL := hyperliquid.MainnetAPIURL
	if testnet {
		baseURL = hyperliquid.TestnetAPIURL
	}

	ctx := context.Background()

	// 创建 Info 客户端（公开API，不需要私钥）
	// skipWS=true: 不需要 WebSocket
	// meta=nil, spotMeta=nil: 会自动获取
	info := hyperliquid.NewInfo(ctx, baseURL, true, nil, nil)

	return &HyperliquidDataSource{
		info: info,
		ctx:  ctx,
		name: "Hyperliquid",
	}
}

// GetName 获取数据源名称
func (h *HyperliquidDataSource) GetName() string {
	return h.name
}

// GetKlines 获取K线数据
func (h *HyperliquidDataSource) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	// 转换 symbol: BTCUSDT -> BTC
	coin := convertSymbolToHyperliquid(symbol)

	// 计算时间范围（最近 limit 个 K线）
	endTime := time.Now().UnixMilli()
	startTime := calculateStartTime(endTime, interval, limit)

	// 获取 Candles 数据
	candles, err := h.info.CandlesSnapshot(h.ctx, coin, interval, startTime, endTime)
	if err != nil {
		log.Printf("⚠️  Hyperliquid GetKlines 失败 [%s %s]: %v", symbol, interval, err)
		return nil, fmt.Errorf("hyperliquid GetKlines failed: %w", err)
	}

	// 转换为 Kline 格式
	klines := make([]Kline, 0, len(candles))
	for _, candle := range candles {
		kline, err := convertCandleToKline(candle)
		if err != nil {
			log.Printf("⚠️  转换 Candle 失败: %v", err)
			continue
		}
		klines = append(klines, kline)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no valid klines returned")
	}

	// 如果返回的数据超过 limit，取最新的 limit 条
	if len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}

	log.Printf("✅ Hyperliquid GetKlines 成功 [%s %s]: %d 条数据", symbol, interval, len(klines))
	return klines, nil
}

// GetTicker 获取ticker数据
func (h *HyperliquidDataSource) GetTicker(symbol string) (*Ticker, error) {
	// 转换 symbol: BTCUSDT -> BTC
	coin := convertSymbolToHyperliquid(symbol)

	// 获取所有 Mids 价格
	mids, err := h.info.AllMids(h.ctx)
	if err != nil {
		log.Printf("⚠️  Hyperliquid GetTicker 失败 [%s]: %v", symbol, err)
		return nil, fmt.Errorf("hyperliquid GetTicker failed: %w", err)
	}

	// 查找对应币种的价格
	priceStr, ok := mids[coin]
	if !ok {
		return nil, fmt.Errorf("price not found for %s (%s)", symbol, coin)
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parse price failed: %w", err)
	}

	ticker := &Ticker{
		Symbol:    symbol,
		LastPrice: price,
		Timestamp: time.Now().Unix(),
	}

	log.Printf("✅ Hyperliquid GetTicker 成功 [%s]: %.2f", symbol, price)
	return ticker, nil
}

// HealthCheck 健康检查
func (h *HyperliquidDataSource) HealthCheck() error {
	// 尝试获取 AllMids 作为健康检查
	_, err := h.info.AllMids(h.ctx)
	if err != nil {
		log.Printf("❌ Hyperliquid 健康检查失败: %v", err)
		return fmt.Errorf("hyperliquid health check failed: %w", err)
	}

	log.Printf("✅ Hyperliquid 健康检查成功")
	return nil
}

// GetLatency 获取延迟
func (h *HyperliquidDataSource) GetLatency() time.Duration {
	start := time.Now()
	_ = h.HealthCheck()
	latency := time.Since(start)

	log.Printf("📊 Hyperliquid 延迟: %v", latency)
	return latency
}

// === Helper functions ===

// convertSymbolToHyperliquid 将 Binance 格式的 symbol 转换为 Hyperliquid 格式
// BTCUSDT -> BTC, ETHUSDT -> ETH
func convertSymbolToHyperliquid(symbol string) string {
	// 去掉 USDT 后缀
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USDT")
	}
	// 如果没有 USDT 后缀，直接返回
	return symbol
}

// convertCandleToKline 将 Hyperliquid Candle 转换为 Kline
func convertCandleToKline(candle hyperliquid.Candle) (Kline, error) {
	var kline Kline

	// 解析字符串字段为 float64
	open, err := strconv.ParseFloat(candle.Open, 64)
	if err != nil {
		return kline, fmt.Errorf("parse Open failed: %w", err)
	}

	high, err := strconv.ParseFloat(candle.High, 64)
	if err != nil {
		return kline, fmt.Errorf("parse High failed: %w", err)
	}

	low, err := strconv.ParseFloat(candle.Low, 64)
	if err != nil {
		return kline, fmt.Errorf("parse Low failed: %w", err)
	}

	close, err := strconv.ParseFloat(candle.Close, 64)
	if err != nil {
		return kline, fmt.Errorf("parse Close failed: %w", err)
	}

	volume, err := strconv.ParseFloat(candle.Volume, 64)
	if err != nil {
		return kline, fmt.Errorf("parse Volume failed: %w", err)
	}

	kline = Kline{
		OpenTime:  candle.Time,      // 开盘时间（毫秒）
		Open:      open,             // 开盘价
		High:      high,             // 最高价
		Low:       low,              // 最低价
		Close:     close,            // 收盘价
		Volume:    volume,           // 成交量
		CloseTime: candle.Timestamp, // 收盘时间（毫秒）
		Trades:    candle.Number,    // 成交笔数
		// Hyperliquid 不提供以下字段，使用默认值 0
		QuoteVolume:         0,
		TakerBuyBaseVolume:  0,
		TakerBuyQuoteVolume: 0,
	}

	return kline, nil
}

// calculateStartTime 根据 interval 和 limit 计算开始时间
func calculateStartTime(endTime int64, interval string, limit int) int64 {
	// 将 interval 转换为毫秒
	var intervalMs int64

	switch interval {
	case "1m":
		intervalMs = 60 * 1000
	case "3m":
		intervalMs = 3 * 60 * 1000
	case "5m":
		intervalMs = 5 * 60 * 1000
	case "15m":
		intervalMs = 15 * 60 * 1000
	case "30m":
		intervalMs = 30 * 60 * 1000
	case "1h":
		intervalMs = 60 * 60 * 1000
	case "2h":
		intervalMs = 2 * 60 * 60 * 1000
	case "4h":
		intervalMs = 4 * 60 * 60 * 1000
	case "8h":
		intervalMs = 8 * 60 * 60 * 1000
	case "12h":
		intervalMs = 12 * 60 * 60 * 1000
	case "1d":
		intervalMs = 24 * 60 * 60 * 1000
	case "3d":
		intervalMs = 3 * 24 * 60 * 60 * 1000
	case "1w":
		intervalMs = 7 * 24 * 60 * 60 * 1000
	case "1M":
		intervalMs = 30 * 24 * 60 * 60 * 1000 // 近似值
	default:
		// 默认使用 15 分钟
		intervalMs = 15 * 60 * 1000
	}

	// 开始时间 = 结束时间 - (limit * interval)
	startTime := endTime - (int64(limit) * intervalMs)

	// 增加 10% 的缓冲（避免时区或边界问题）
	startTime -= intervalMs * int64(limit) / 10

	return startTime
}
