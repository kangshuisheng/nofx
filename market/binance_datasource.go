package market

import (
	"fmt"
	"log"
	"time"
)

// BinanceDataSource 封装 Binance 作为数据源
type BinanceDataSource struct {
	client *APIClient
	name   string
}

// NewBinanceDataSource 创建 Binance 数据源实例
func NewBinanceDataSource() *BinanceDataSource {
	return &BinanceDataSource{
		client: NewAPIClient(),
		name:   "Binance",
	}
}

// GetName 获取数据源名称
func (b *BinanceDataSource) GetName() string {
	return b.name
}

// GetKlines 获取K线数据
func (b *BinanceDataSource) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	klines, err := b.client.GetKlines(symbol, interval, limit)
	if err != nil {
		log.Printf("⚠️  Binance GetKlines 失败 [%s %s]: %v", symbol, interval, err)
		return nil, fmt.Errorf("binance GetKlines failed: %w", err)
	}

	log.Printf("✅ Binance GetKlines 成功 [%s %s]: %d 条数据", symbol, interval, len(klines))
	return klines, nil
}

// GetTicker 获取ticker数据
func (b *BinanceDataSource) GetTicker(symbol string) (*Ticker, error) {
	price, err := b.client.GetCurrentPrice(symbol)
	if err != nil {
		log.Printf("⚠️  Binance GetTicker 失败 [%s]: %v", symbol, err)
		return nil, fmt.Errorf("binance GetTicker failed: %w", err)
	}

	ticker := &Ticker{
		Symbol:    symbol,
		LastPrice: price,
		Timestamp: time.Now().Unix(),
	}

	log.Printf("✅ Binance GetTicker 成功 [%s]: %.2f", symbol, price)
	return ticker, nil
}

// HealthCheck 健康检查
func (b *BinanceDataSource) HealthCheck() error {
	_, err := b.client.GetExchangeInfo()
	if err != nil {
		log.Printf("❌ Binance 健康检查失败: %v", err)
		return fmt.Errorf("binance health check failed: %w", err)
	}

	log.Printf("✅ Binance 健康检查成功")
	return nil
}

// GetLatency 获取延迟
func (b *BinanceDataSource) GetLatency() time.Duration {
	start := time.Now()
	_ = b.HealthCheck()
	latency := time.Since(start)

	log.Printf("📊 Binance 延迟: %v", latency)
	return latency
}
