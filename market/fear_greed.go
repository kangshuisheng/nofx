package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FearGreedIndex 恐慌贪婪指数数据
type FearGreedIndex struct {
	Value           int       `json:"value"`             // 0-100 (0=极度恐慌, 100=极度贪婪)
	ValueText       string    `json:"value_text"`        // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	Timestamp       time.Time `json:"timestamp"`         // 数据时间戳
	LastUpdate      time.Time `json:"last_update"`       // 最后更新时间
	TimeUntilUpdate int       `json:"time_until_update"` // 距离下次更新的秒数
}

// FearGreedAPIResponse Alternative.me API 响应结构
type FearGreedAPIResponse struct {
	Name string `json:"name"`
	Data []struct {
		Value               string `json:"value"`
		ValueClassification string `json:"value_classification"`
		Timestamp           string `json:"timestamp"`
		TimeUntilUpdate     string `json:"time_until_update,omitempty"`
	} `json:"data"`
	Metadata struct {
		Error string `json:"error,omitempty"`
	} `json:"metadata"`
}

// FearGreedClient 恐慌贪婪指数客户端
type FearGreedClient struct {
	apiURL      string
	httpClient  *http.Client
	cache       *FearGreedIndex
	cacheExpiry time.Time
}

// NewFearGreedClient 创建恐慌贪婪指数客户端
func NewFearGreedClient() *FearGreedClient {
	return &FearGreedClient{
		apiURL: "https://api.alternative.me/fng/?limit=1",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetFearGreedIndex 获取当前恐慌贪婪指数
func (c *FearGreedClient) GetFearGreedIndex() (*FearGreedIndex, error) {
	// 如果缓存有效（4小时内），直接返回
	// 注意：alternative.me 的恐慌指数每天更新一次，使用较长的缓存时间避免被限流
	if c.cache != nil && time.Now().Before(c.cacheExpiry) {
		return c.cache, nil
	}

	// 请求 API
	resp, err := c.httpClient.Get(c.apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求恐慌贪婪指数失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误状态码: %d", resp.StatusCode)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var apiResp FearGreedAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("API 返回空数据")
	}

	// 提取数据
	data := apiResp.Data[0]

	// 转换 value
	var value int
	fmt.Sscanf(data.Value, "%d", &value)

	// 转换 timestamp
	var timestamp int64
	fmt.Sscanf(data.Timestamp, "%d", &timestamp)

	// 构建结果
	index := &FearGreedIndex{
		Value:      value,
		ValueText:  data.ValueClassification,
		Timestamp:  time.Unix(timestamp, 0),
		LastUpdate: time.Now(),
	}

	// 缓存结果（4小时）
	// alternative.me 恐慌指数每天更新一次，长缓存避免API限流
	c.cache = index
	c.cacheExpiry = time.Now().Add(4 * time.Hour)

	return index, nil
}

// GetMarketSentiment 获取市场情绪描述
func (fgi *FearGreedIndex) GetMarketSentiment() string {
	if fgi.Value <= 20 {
		return fmt.Sprintf("极度恐慌 (Fear & Greed: %d/100)", fgi.Value)
	} else if fgi.Value <= 40 {
		return fmt.Sprintf("恐慌 (Fear & Greed: %d/100)", fgi.Value)
	} else if fgi.Value <= 60 {
		return fmt.Sprintf("中性 (Fear & Greed: %d/100)", fgi.Value)
	} else if fgi.Value <= 80 {
		return fmt.Sprintf("贪婪 (Fear & Greed: %d/100)", fgi.Value)
	} else {
		return fmt.Sprintf("极度贪婪 (Fear & Greed: %d/100)", fgi.Value)
	}
}

// GetTradingSuggestion 根据恐慌贪婪指数给出交易建议
func (fgi *FearGreedIndex) GetTradingSuggestion() string {
	if fgi.Value <= 20 {
		return "市场极度恐慌，可能是买入机会（但需确认支撑位和底部形态）"
	} else if fgi.Value <= 40 {
		return "市场恐慌，关注超卖信号和潜在反弹机会"
	} else if fgi.Value <= 60 {
		return "市场情绪中性，根据技术分析做决策"
	} else if fgi.Value <= 80 {
		return "市场贪婪，注意顶部风险，关注超买信号"
	} else {
		return "市场极度贪婪，警惕回调风险，可考虑部分止盈或做空"
	}
}

// IsExtremeFear 是否极度恐慌（<= 20）
func (fgi *FearGreedIndex) IsExtremeFear() bool {
	return fgi.Value <= 20
}

// IsExtremeGreed 是否极度贪婪（>= 80）
func (fgi *FearGreedIndex) IsExtremeGreed() bool {
	return fgi.Value >= 80
}
