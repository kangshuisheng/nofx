package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// 预编译正则表达式（性能优化：避免每次调用时重新编译）
var (
	// ✅ 安全的正則：精確匹配 ```json 代碼塊
	// 使用反引號 + 拼接避免轉義問題
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// 新增：XML标签提取（支持思维链中包含任何字符）
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // 历史最高收益率（百分比）
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"`           // 持仓更新时间戳（毫秒）
	StopLoss         float64 `json:"stop_loss,omitempty"`   // 止损价格（用于推断平仓原因）
	TakeProfit       float64 `json:"take_profit,omitempty"` // 止盈价格（用于推断平仓原因）
}

// OpenOrderInfo represents an open order for AI decision context
type OpenOrderInfo struct {
	Symbol       string  `json:"symbol"`        // Trading pair
	OrderID      int64   `json:"order_id"`      // Order ID
	Type         string  `json:"type"`          // Order type: STOP_MARKET, TAKE_PROFIT_MARKET, LIMIT, MARKET
	Side         string  `json:"side"`          // Order side: BUY, SELL
	PositionSide string  `json:"position_side"` // Position side: LONG, SHORT, BOTH
	Quantity     float64 `json:"quantity"`      // Order quantity
	Price        float64 `json:"price"`         // Limit order price (for limit orders)
	StopPrice    float64 `json:"stop_price"`    // Trigger price (for stop-loss/take-profit orders)
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // 未实现盈亏
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	OpenOrders      []OpenOrderInfo         `json:"open_orders"` // List of open orders for AI context
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis，包含 RecentTrades）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	TakerFeeRate    float64                 `json:"-"` // Taker fee rate (from config, default 0.0004)
	MakerFeeRate    float64                 `json:"-"` // Maker fee rate (from config, default 0.0002)
	Timeframes      []string                `json:"-"` // K线时间线配置（从trader配置读取）

	// ⚡ 新增：全局市場情緒數據（VIX 恐慌指數 + 美股狀態）
	GlobalSentiment *market.MarketSentiment `json:"-"` // 全局風險情緒（免費來源：Yahoo Finance + Alpha Vantage）
}

// Decision AI的交易决策
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	EntryPrice      float64 `json:"entry_price,omitempty"` // 限价单价格 (0表示市价)

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // 用于 partial_close (0-100)

	// 通用参数
	Confidence int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
	// AIRequestDurationMs 记录 AI API 调用耗时（毫秒）方便排查延迟问题
	AIRequestDurationMs int64 `json:"ai_request_duration_ms,omitempty"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient mcp.AIClient, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	fetchStart := time.Now()
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	fetchDuration := time.Since(fetchStart).Seconds()
	log.Printf("⏱️  市場數據獲取耗時: %.2fs（%d 個幣種）", fetchDuration, len(ctx.MarketDataMap))

	// 1.5. ⚡ 獲取全局市場情緒（VIX + 美股，免費來源）
	alphaVantageKey := os.Getenv("ALPHA_VANTAGE_API_KEY") // 可選，用於美股數據（免費 500 calls/day）
	sentiment, err := market.FetchMarketSentiment(alphaVantageKey)
	if err != nil {
		// 非關鍵數據，失敗不阻塞主流程
		log.Printf("⚠️  獲取全局市場情緒失敗（不影響交易）: %v", err)
	} else {
		ctx.GlobalSentiment = sentiment
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.Positions)

	// 无论是否有错误，都要保存 SystemPrompt 和 UserPrompt（用于调试和决策未执行后的问题定位）
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // 保存系统prompt
		decision.UserPrompt = userPrompt     // 保存输入prompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
	}

	if err != nil {
		return decision, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // 保存系统prompt
	decision.UserPrompt = userPrompt     // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// ✅ 优化：并发获取市场数据（提升性能 5-10x）
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	// 并发获取市场数据
	type marketDataResult struct {
		symbol string
		data   *market.Data
		err    error
	}

	resultChan := make(chan marketDataResult, len(symbolSet))
	var wg sync.WaitGroup

	for symbol := range symbolSet {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			data, err := market.Get(sym, ctx.Timeframes)
			resultChan <- marketDataResult{symbol: sym, data: data, err: err}
		}(symbol)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果并应用过滤
	const minOIThresholdMillions = 15.0 // 可調整：15M(保守) / 10M(平衡) / 8M(寬鬆) / 5M(激進)

	// ✅ 錯誤統計
	failedSymbols := []string{}
	filteredSymbols := []string{}

	for result := range resultChan {
		if result.err != nil {
			// 收集失敗的幣種（稍後統一報告）
			failedSymbols = append(failedSymbols, result.symbol)
			continue
		}

		data := result.data
		symbol := result.symbol

		// ⚠️ 流动性过滤：持仓价值低于阈值的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < minOIThresholdMillions {
				filteredSymbols = append(filteredSymbols, symbol)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// ✅ 統一報告結果
	totalSymbols := len(symbolSet)
	successCount := len(ctx.MarketDataMap)
	log.Printf("📊 市場數據獲取完成：成功 %d/%d", successCount, totalSymbols)

	if len(failedSymbols) > 0 {
		log.Printf("⚠️  數據獲取失敗 (%d): %v", len(failedSymbols), failedSymbols)
	}

	if len(filteredSymbols) > 0 {
		log.Printf("🔍 流動性過濾 (%d): %v", len(filteredSymbols), filteredSymbols)
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// 获取基础prompt（使用指定的模板）
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")

	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) string {
	var sb strings.Builder

	if templateName == "" {
		templateName = "default"
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// 如果模板不存在，记录错误并使用 default
		log.Printf("⚠️  提示词模板 '%s' 不存在，使用 default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// 如果连 default 都不存在，使用内置简化版本
			log.Printf("❌ 无法加载任何提示词模板，使用内置简化版本")
			sb.WriteString("你是专业的加密货币交易AI。请根据市场数据做出交易决策。\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// 2. 硬约束（风险控制）- 动态生成
	sb.WriteString("# 硬约束（绝对风控法则）\n\n")
	sb.WriteString(fmt.Sprintf("1. **最大单笔亏损**: **任何单笔交易的潜在亏损不得超过账户净值的2%%** (后端代码强制验证)。你的计算目标应为1.8%%以确保通过。\n"))
	sb.WriteString(fmt.Sprintf("2. **最大仓位价值**: \n   - **山寨币**: 名义价值不得超过账户净值的**75%%** (≤ %.2f USDT)\n   - **BTC/ETH**: 名义价值不得超过账户净值的**85%%** (≤ %.2f USDT)\n", accountEquity*0.75, accountEquity*0.85))
	sb.WriteString("3. **最多持仓**: 3个币种\n")
	sb.WriteString(fmt.Sprintf("4. **杠杆限制**: **山寨币最大%dx** | **BTC/ETH最大%dx**\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString("5. **保证金**: 总使用率 ≤ 90%\n\n")

	// 🚨 增强验证机制说明
	sb.WriteString("## 🛡️ 增强验证机制\n\n")
	sb.WriteString("系统现在使用多层验证机制确保交易安全：\n")
	sb.WriteString("1. **基础验证**: 检查字段完整性、数值范围、杠杆限制\n")
	sb.WriteString("2. **风险计算**: 精确计算潜在亏损和风险比例\n")
	sb.WriteString("3. **智能建议**: 提供优化建议和替代方案\n")
	sb.WriteString("4. **风险评级**: 自动评估交易风险等级 (低/中/高)\n\n")
	sb.WriteString("⚠️ **重要**: 如果验证失败，系统会返回详细错误信息，请根据建议调整参数\n\n")

	// 6. 开仓金额：根据账户规模动态提示（使用统一的配置规则）
	minBTCETH := calculateMinPositionSize("BTCUSDT", accountEquity)

	// 根据账户规模生成不同的提示语
	var btcEthHint string
	if accountEquity < btcEthSizeRules[1].MinEquity {
		// 小账户模式（< 20U）
		btcEthHint = fmt.Sprintf(" | BTC/ETH≥%.0f USDT (⚠️ 小账户模式，降低门槛)", minBTCETH)
	} else if accountEquity < btcEthSizeRules[2].MinEquity {
		// 中型账户（20-100U）
		btcEthHint = fmt.Sprintf(" | BTC/ETH≥%.0f USDT (根据账户规模动态调整)", minBTCETH)
	} else {
		// 大账户（≥100U）
		btcEthHint = fmt.Sprintf(" | BTC/ETH≥%.0f USDT", minBTCETH)
	}

	sb.WriteString("6. 开仓金额: 山寨币≥12 USDT")
	sb.WriteString(btcEthHint)
	sb.WriteString("\n\n")

	// ⚠️ 重要提醒：防止 AI 误读市场数据中的数字
	sb.WriteString("⚠️ **重要提醒：计算 position_size_usd 的正确方法**\n\n")
	sb.WriteString(fmt.Sprintf("- 当前账户净值：**%.2f USDT**\n", accountEquity))
	sb.WriteString(fmt.Sprintf("- 山寨币开仓范围：**12 - %.0f USDT** (最大0.75倍净值)\n", accountEquity*0.75))
	sb.WriteString(fmt.Sprintf("- BTC/ETH开仓范围：**%.0f - %.0f USDT** (最大0.85倍净值)\n", minBTCETH, accountEquity*0.85))
	sb.WriteString("- ❌ **不要使用市场数据中的任何数字**（如 Open Interest 合约数、Volume、价格等）作为 position_size_usd\n")
	sb.WriteString("- ✅ **position_size_usd 必须根据账户净值和上述范围计算**\n")
	sb.WriteString("- ✅ **系统会自动验证所有计算，确保风险控制在安全范围内**\n\n")

	// 3. 输出格式 - 动态生成
	sb.WriteString("# 输出格式 (严格遵守)\n\n")
	sb.WriteString("**必须使用XML标签 <reasoning> 和 <decision> 标签分隔思维链和决策JSON，避免解析错误**\n\n")
	sb.WriteString("## 格式要求\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("你的思维链分析...\n")
	sb.WriteString("- 简洁分析你的思考过程 \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"entry_price\": 65000, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"下跌趋势+MACD死叉\"},\n", btcEthLeverage, accountEquity*0.85))
	sb.WriteString("  {\"symbol\": \"SOLUSDT\", \"action\": \"update_stop_loss\", \"new_stop_loss\": 155, \"reasoning\": \"移动止损至保本位\"},\n")
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"止盈离场\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## 字段说明\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | update_stop_loss | update_take_profit | partial_close | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100（开仓建议≥80）\n")
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n")
	sb.WriteString("- **限价单必填**: `entry_price` (设置 > 0 的价格即为限价单，0 为市价单)\n")
	sb.WriteString("- update_stop_loss 时必填: new_stop_loss (注意是 new_stop_loss，不是 stop_loss)\n")
	sb.WriteString("- update_take_profit 时必填: new_take_profit (注意是 new_take_profit，不是 take_profit)\n")
	sb.WriteString("- partial_close 时必填: close_percentage (1-100)\n\n")
	sb.WriteString("## 🛡️ 未成交挂单提醒\n\n")
	sb.WriteString("在「当前持仓」部分，你会看到每个持仓的挂单状态：\n\n")
	sb.WriteString("- 🛡️ **止损单**: 表示该持仓已有止损保护\n")
	sb.WriteString("- 🎯 **止盈单**: 表示该持仓已设置止盈目标\n")
	sb.WriteString("- ⚠️ **该持仓没有止损保护！**: 表示该持仓缺少止损单，需要立即设置\n\n")
	sb.WriteString("**重要**: \n")
	sb.WriteString("- ✅ 如果看到 🛡️ 止损单已存在，且你想调整止损价格，仍可使用 `update_stop_loss` 动作（系统会自动取消旧单并设置新单）\n")
	sb.WriteString("- ⚠️ 如果看到 🛡️ 止损单已存在，且当前止损价格合理，**不要重复发送相同的 update_stop_loss 指令**\n")
	sb.WriteString("- 🚨 如果看到 ⚠️ **该持仓没有止损保护！**，必须立即使用 `update_stop_loss` 设置止损，否则风险极高\n")
	sb.WriteString("- 同样规则适用于 `update_take_profit` 和 🎯 止盈单\n\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据核心）
// 这是一个“总指挥”函数，负责拼装各个模块的情报
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 1. 抬头信息：时间与运行状态
	sb.WriteString(fmt.Sprintf("# 📅 交易简报 | 时间: %s | 运行时长: %d分钟 | 决策周期: #%d\n\n",
		ctx.CurrentTime, ctx.RuntimeMinutes, ctx.CallCount))

	// 2. 宏观情报：先看天吃饭 (BTC + VIX)
	sb.WriteString(buildMarketContextSection(ctx))

	// 3. 账户风控：告诉 AI 具体的数字限制
	sb.WriteString(buildAccountSection(ctx))

	// 4. 持仓巡检：这是最关键的部分，集成了 Go 的状态判断逻辑
	sb.WriteString(buildPositionsSection(ctx))

	// 5. 猎物雷达：候选币种数据
	sb.WriteString(buildCandidatesSection(ctx))

	// 6. 历史表现与结尾指令
	sb.WriteString(buildPerformanceAndFooter(ctx))

	return sb.String()
}

// buildMarketContextSection 构建宏观市场数据部分
func buildMarketContextSection(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 🌍 1. 宏观市场情报 (Global Context)\n")
	sb.WriteString("> 这里的状态决定了是否允许开新仓 (Long/Short)。\n\n")

	// 1.1 VIX 恐慌指数 (如有)
	// 1.1 VIX 恐慌指数 (如有)
	if ctx.GlobalSentiment != nil {
		sb.WriteString(fmt.Sprintf("- **市场情绪 (VIX)**: %.2f [%s]\n",
			ctx.GlobalSentiment.VIX, ctx.GlobalSentiment.FearLevel))
		sb.WriteString(fmt.Sprintf("  👉 **风控建议**: %s\n", ctx.GlobalSentiment.Recommendation))
	}

	// 1.2 恐慌贪婪指数 (Fear & Greed Index) - 新增
	// 从 BTCUSDT 数据中获取 (因为它是全局指标，每个 Data 都有)
	if btcData, ok := ctx.MarketDataMap["BTCUSDT"]; ok && btcData.FearGreedIndex != nil {
		fg := btcData.FearGreedIndex
		sb.WriteString(fmt.Sprintf("- **Fear & Greed Index**: %d [%s]\n", fg.Value, fg.Classification))

		// 简单的行动建议
		var advice string
		if fg.Value < 20 {
			advice = "极度恐慌 (Extreme Fear) -> 寻找超跌反弹机会"
		} else if fg.Value > 80 {
			advice = "极度贪婪 (Extreme Greed) -> 警惕顶部反转"
		} else {
			advice = "情绪中性 -> 依赖技术面"
		}
		sb.WriteString(fmt.Sprintf("  👉 **AI参考**: %s\n", advice))
	}

	// 1.2 BTC 领头羊状态
	if btcData, ok := ctx.MarketDataMap["BTCUSDT"]; ok {
		trendStr := "震荡"
		if btcData.CurrentPrice > btcData.CurrentEMA20 {
			trendStr = "多头偏强 (Price > EMA20)"
		} else {
			trendStr = "空头偏强 (Price < EMA20)"
		}

		sb.WriteString(fmt.Sprintf("- **BTC 状态**: 价格 %.2f | 1h: %+.2f%% | 4h: %+.2f%%\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h))
		sb.WriteString(fmt.Sprintf("  👉 **大盘趋势**: %s | MACD: %.2f | RSI: %.2f\n",
			trendStr, btcData.CurrentMACD, btcData.CurrentRSI7))
	} else {
		sb.WriteString("- **BTC 状态**: 数据获取失败，请谨慎操作。\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildAccountSection 构建账户与硬性风控部分
func buildAccountSection(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 💼 2. 账户资金与硬性风控 (Risk Limits)\n")
	sb.WriteString("> 所有开仓指令必须通过以下验证，否则会被拒绝。\n\n")

	// 计算具体的风控数值，直接告诉 AI 结果
	maxRiskUSD := ctx.Account.TotalEquity * 0.03 // 3% 单笔最大亏损

	// 获取 BTC 和 山寨 的具体仓位上限
	// minBTCSize := calculateMinPositionSize("BTCUSDT", ctx.Account.TotalEquity)
	maxPosBTC := ctx.Account.TotalEquity * 0.85
	maxPosAlt := ctx.Account.TotalEquity * 0.75

	// 🔒 测试阶段：设置名义价值上限
	// 名义价值 = 保证金 × 杠杆，因此保证金上限 = 名义价值上限 / 杠杆
	// BTC/ETH: 5x 杠杆 → 保证金上限 = 80 / 5 = 16 USDT (名义价值 80 USDT)
	// 山寨币: 3x 杠杆 → 保证金上限 = 60 / 3 = 20 USDT (名义价值 60 USDT)
	maxNotionalValueBTC := 80.0 // BTC/ETH 名义价值上限（USDT）
	maxNotionalValueAlt := 60.0 // 山寨币名义价值上限（USDT）

	// 动态计算保证金上限（根据杠杆倍数）
	btcEthLeverage := float64(ctx.BTCETHLeverage)   // 默认 5x
	altcoinLeverage := float64(ctx.AltcoinLeverage) // 默认 3x

	maxMarginBTC := maxNotionalValueBTC / btcEthLeverage  // 80 / 5 = 16 USDT
	maxMarginAlt := maxNotionalValueAlt / altcoinLeverage // 60 / 3 = 20 USDT

	if maxPosBTC > maxMarginBTC {
		maxPosBTC = maxMarginBTC
	}
	if maxPosAlt > maxMarginAlt {
		maxPosAlt = maxMarginAlt
	}

	sb.WriteString(fmt.Sprintf("- **账户净值**: %.2f USDT | **可用余额**: %.2f USDT\n",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- **持仓占用**: %d / 3 个位置\n", ctx.Account.PositionCount))

	sb.WriteString("- **本轮开仓限制 (Hard Constraints)**:\n")
	sb.WriteString(fmt.Sprintf("  1. **最大亏损 (Risk)**: 单笔不得超过 **%.2f USDT** (净值的 3%%)\n", maxRiskUSD))
	sb.WriteString(fmt.Sprintf("  2. **BTC/ETH 开仓价值**: 24 - %.0f USDT\n", maxPosBTC))
	sb.WriteString(fmt.Sprintf("  3. **山寨币开仓价值**: 12 - %.0f USDT\n", maxPosAlt))
	sb.WriteString("\n")
	return sb.String()
}

func buildPositionsSection(ctx *Context) string {
	if len(ctx.Positions) == 0 {
		return "## 🛡️ 3. 当前持仓管理 (Positions)\n- 目前空仓 (No Positions)，请专注于寻找猎物。\n\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 🛡️ 3. 当前持仓管理 (%d 个持仓)\n", len(ctx.Positions)))
	sb.WriteString("> 任务：检查每个持仓的 [系统判定状态] 并执行相应的 [AI 行动指南]。\n\n")

	for i, pos := range ctx.Positions {
		// 1. 获取基础数据
		marketData := ctx.MarketDataMap[pos.Symbol]

		// 2. 查找当前止损/止盈单
		var currentSL, currentTP float64
		hasSL, hasTP := false, false
		for _, order := range ctx.OpenOrders {
			if order.Symbol != pos.Symbol {
				continue
			}
			if (pos.Side == "long" && order.Side == "SELL") || (pos.Side == "short" && order.Side == "BUY") {
				if order.Type == "STOP_MARKET" || order.Type == "STOP" {
					currentSL = order.StopPrice
					hasSL = true
				}
				if order.Type == "TAKE_PROFIT_MARKET" || order.Type == "TAKE_PROFIT" {
					currentTP = order.StopPrice
					hasTP = true
				}
			}
		}

		// 3. 计算管理状态 (调用 Go 的逻辑)
		state := "NO_STOP_LOSS"
		rRatio := 0.0
		if hasSL && marketData != nil {
			state, rRatio = calculateManagementState(pos, currentSL, marketData)
		}

		// 4. 生成具体的行动指南 (将 Go 状态翻译成人话)
		actionGuide := ""
		statusIcon := ""

		switch state {
		case "NO_STOP_LOSS":
			statusIcon = "🚨"
			actionGuide = "**极度危险**:该持仓没有止损!请立即输出 `update_stop_loss` (建议距离 ATR*3,中长线策略)。"
		case "STAGE_1_INITIAL_RISK":
			statusIcon = "🥚"
			actionGuide = "**孵化期**：R:R < 0.8。除非价格跌破关键技术结构，否则 **HOLD**。给交易呼吸空间。"
		case "STAGE_2_RISK_REMOVAL":
			statusIcon = "🛡️"
			// 检查是否真保本了
			isSafe := (pos.Side == "long" && currentSL >= pos.EntryPrice) || (pos.Side == "short" && currentSL <= pos.EntryPrice)
			if isSafe {
				actionGuide = "**安全期**：风险已移除。保持持有，等待利润奔跑。"
			} else {
				actionGuide = "**行动请求**：系统判定应移除风险。请输出 `update_stop_loss` 将止损移至入场价附近 (Breakeven)。"
			}
		case "STAGE_3_TRAILING":
			statusIcon = "💰"
			actionGuide = "**获利期**：R:R > 1.5。请检查是否满足 `partial_close` (R:R>2.5) 或根据 ATR 收紧止损来锁定利润。"
		default:
			statusIcon = "❓"
			actionGuide = "数据不足，建议 HOLD。"
		}

		// 5. 拼装显示
		posValue := math.Abs(pos.Quantity) * pos.MarkPrice
		sb.WriteString(fmt.Sprintf("### %d. %s %s %s (价值: %.1f U)\n",
			i+1, statusIcon, pos.Symbol, strings.ToUpper(pos.Side), posValue))

		sb.WriteString(fmt.Sprintf("   - **盈亏**: %+.2f%% (R:R = %.2f)\n", pos.UnrealizedPnLPct, rRatio))
		sb.WriteString(fmt.Sprintf("   - **价格**: 入场 %.4f | 当前 %.4f | 止损 %.4f\n", pos.EntryPrice, pos.MarkPrice, currentSL))

		if hasTP {
			sb.WriteString(fmt.Sprintf("   - **止盈**: %.4f\n", currentTP))
		}

		sb.WriteString(fmt.Sprintf("   - **状态**: `%s`\n", state))
		sb.WriteString(fmt.Sprintf("   👉 **AI 行动指南**: %s\n", actionGuide)) // 关键行：直接指导 AI

		// 附带市场数据供验证
		if marketData != nil {
			sb.WriteString("\n   [参考数据]\n")
			sb.WriteString(market.Format(marketData))
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// buildCandidatesSection 构建候选币种部分
func buildCandidatesSection(ctx *Context) string {
	// 1. 建立持仓索引，用于过滤
	holdingMap := make(map[string]bool)
	for _, pos := range ctx.Positions {
		holdingMap[pos.Symbol] = true
	}

	var sb strings.Builder
	sb.WriteString("## 🎯 4. 猎物扫描 (Candidate Setup)\n\n")

	validCount := 0
	for _, coin := range ctx.CandidateCoins {
		// 过滤掉已经持有的币种
		if holdingMap[coin.Symbol] {
			continue
		}

		marketData, ok := ctx.MarketDataMap[coin.Symbol]
		if !ok {
			continue
		}

		validCount++
		sourceTag := "AI500"
		if len(coin.Sources) > 0 {
			sourceTag = strings.Join(coin.Sources, "+")
		}

		sb.WriteString(fmt.Sprintf("### [%d] %s (%s)\n", validCount, coin.Symbol, sourceTag))

		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}

	if validCount == 0 {
		sb.WriteString("(当前无符合条件的候选币种，或候选币种已全部在持仓中)\n\n")
	}
	return sb.String()
}

// buildPerformanceAndFooter 构建历史记录和结尾
func buildPerformanceAndFooter(ctx *Context) string {
	var sb strings.Builder

	// 历史表现
	if ctx.Performance != nil {
		// 这里使用简单的 JSON 序列化再解析有点绕，但为了保持类型兼容先这样做
		// 理想情况下 ctx.Performance 应该是一个具体的 Struct 类型
		type PerformanceData struct {
			SharpeRatio  float64               `json:"sharpe_ratio"`
			RecentTrades []logger.TradeOutcome `json:"recent_trades"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📜 历史战绩参考 (Sharpe: %.2f)\n", perfData.SharpeRatio))
				if len(perfData.RecentTrades) > 0 {
					sb.WriteString("最近 3 笔交易:\n")
					// 只显示最近 3 笔，节省 Token，让 AI 更有重点
					count := 0
					for _, trade := range perfData.RecentTrades {
						if count >= 3 {
							break
						}
						icon := "✅"
						if trade.PnL < 0 {
							icon = "❌"
						}
						sb.WriteString(fmt.Sprintf("- %s %s %s: %+.2f%%\n", icon, trade.Symbol, trade.Side, trade.PnLPct))
						count++
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	sb.WriteString("---\n")
	sb.WriteString("现在，请开始你的思维链分析 `<reasoning>`，然后输出 JSON 决策。\n")
	sb.WriteString("记住：**少动多看，只打高分牌**。\n")

	return sb.String()
}

// calculateManagementState 计算持仓的管理状态和 R:R 比例
func calculateManagementState(pos PositionInfo, currentStopLossPrice float64, marketData *market.Data) (string, float64) {
	if currentStopLossPrice == 0 {
		return "NO_STOP_LOSS", 0
	}

	if marketData == nil || marketData.LongerTermContext == nil || marketData.LongerTermContext.ATR14 == 0 {
		return "CALC_PENDING", 0
	}

	// 1. 计算初始风险距离 (总是正数)
	initialRisk := math.Abs(pos.EntryPrice - currentStopLossPrice)
	if initialRisk == 0 {
		initialRisk = marketData.LongerTermContext.ATR14 // 防止除以0
	}

	// 2. ✅ 修复：计算当前盈利距离 (区分方向，亏损为负数)
	var currentProfitDist float64
	if pos.Side == "long" {
		currentProfitDist = pos.MarkPrice - pos.EntryPrice
	} else {
		// 空单：入场价 - 当前价 (如果当前价更高，结果为负)
		currentProfitDist = pos.EntryPrice - pos.MarkPrice
	}

	// 3. 计算 R:R (亏损时 R:R 为负数)
	rRatio := currentProfitDist / initialRisk

	// 4. 判断是否已保本
	isBreakeven := (pos.Side == "long" && currentStopLossPrice >= pos.EntryPrice) ||
		(pos.Side == "short" && currentStopLossPrice <= pos.EntryPrice)

	// 5. 精细状态判断
	var state string
	switch {
	case rRatio < 0.3:
		// 包含负数的情况 (亏损)，都属于孵化期
		state = "STAGE_1_INITIAL_RISK"

	case rRatio >= 0.3 && rRatio < 0.8:
		// 小赚
		state = "STAGE_1_INITIAL_RISK"

	case rRatio >= 0.8 && rRatio < 1.0:
		// 接近保本
		state = "STAGE_2_RISK_REMOVAL"

	case rRatio >= 1.0 && rRatio < 1.5:
		// 已保本或该保本了
		if isBreakeven {
			state = "STAGE_2_RISK_REMOVAL"
		} else {
			// 还没保本，但利润够了，提示去保本
			state = "STAGE_2_RISK_REMOVAL"
		}

	case rRatio >= 1.5:
		// 大赚
		state = "STAGE_3_TRAILING"

	default:
		state = "STAGE_1_INITIAL_RISK"
	}

	return state, rRatio
}

// CheckEmergencyExit 检查是否需要紧急离场（趋势破坏）
// 返回值: (是否需要平仓, 原因)
//
// 🔧 中长线策略优化: 完全禁用硬风控
// 理由:
// 1. 中长线策略不在意短期波动,给交易足够的呼吸空间
// 2. 止损已调整为ATR*3,有足够的容错空间
// 3. 完全交给AI根据大周期趋势判断,避免被正常回调扫出
func CheckEmergencyExit(pos PositionInfo, marketData *market.Data) (bool, string) {
	// 中长线策略: 完全禁用紧急平仓,交给AI决策
	// 如果方向错了,通过止损或AI主动平仓处理
	return false, ""
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, currentPositions []PositionInfo) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w", err)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, currentPositions); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 方法1: 优先尝试提取 <reasoning> 标签内容
	if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
		log.Printf("✓ 使用 <reasoning> 标签提取思维链")
		return strings.TrimSpace(match[1])
	}

	// 方法2: 如果没有 <reasoning> 标签，但有 <decision> 标签，提取 <decision> 之前的内容
	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		log.Printf("✓ 提取 <decision> 标签之前的内容作为思维链")
		return strings.TrimSpace(response[:decisionIdx])
	}

	// 方法3: 后备方案 - 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		log.Printf("⚠️  使用旧版格式（[ 字符分离）提取思维链")
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到任何标记，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 预清洗：去零宽/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 关键修复 (Critical Fix)：在正则匹配之前就先修复全角字符！
	// 否则正则表达式 \[ 无法匹配全角的 ［
	s = fixMissingQuotes(s)

	// 方法1: 优先尝试从 <decision> 标签中提取
	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		log.Printf("✓ 使用 <decision> 标签提取JSON")
	} else {
		// 后备方案：使用整个响应
		jsonPart = s
		log.Printf("⚠️  未找到 <decision> 标签，使用全文搜索JSON")
	}

	// 修复 jsonPart 中的全角字符
	jsonPart = fixMissingQuotes(jsonPart)

	// 1) 优先从 ```json 代码块中提取
	if m := reJSONFence.FindStringSubmatch(jsonPart); len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // 把 "[ {" 规整为 "[{"
		jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
		}
		return decisions, nil
	}

	// 2) 退而求其次 (Fallback)：全文寻找首个对象数组
	// 注意：此时 jsonPart 已经过 fixMissingQuotes()，全角字符已转换为半角
	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		// 🔧 安全回退 (Safe Fallback)：当AI只输出思维链没有JSON时，生成保底决策（避免系统崩溃）
		log.Printf("⚠️  [SafeFallback] AI未输出JSON决策，进入安全等待模式 (AI response without JSON, entering safe wait mode)")

		// 提取思维链摘要（最多 240 字符）
		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// 生成保底决策：所有币种进入 wait 状态
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("模型未输出结构化JSON决策，进入安全等待；摘要：%s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	// 🔧 规整格式（此时全角字符已在前面修复过）
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）

	// 🔧 验证 JSON 格式（检测常见错误）
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
	}

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	return jsonStr
}

// validateJSONFormat validates JSON format and detects common errors
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// Allow any whitespace (including zero-width) between [ and {
	if !reArrayHead.MatchString(trimmed) {
		// Check if it's a pure number/range array (common error)
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// Check for range symbol ~ (common LLM error)
	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	// Check for thousands separators (like 98,000) but skip string values
	// Parse through JSON and only check numeric contexts
	if err := checkThousandsSeparatorsOutsideStrings(jsonStr); err != nil {
		return err
	}

	return nil
}

// checkThousandsSeparatorsOutsideStrings checks for thousands separators in JSON numbers
// but ignores commas inside string values
func checkThousandsSeparatorsOutsideStrings(jsonStr string) error {
	inString := false
	escaped := false

	for i := 0; i < len(jsonStr)-4; i++ {
		// Track string boundaries
		if jsonStr[i] == '"' && !escaped {
			inString = !inString
		}
		escaped = (jsonStr[i] == '\\' && !escaped)

		// Skip if we're inside a string value
		if inString {
			continue
		}

		// Check for pattern: digit, comma, 3 digits
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousands separator commas, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, currentPositions []PositionInfo) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, currentPositions); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// positionSizeConfig 定义账户规模分层配置
type positionSizeConfig struct {
	MinEquity float64 // 账户最小净值阈值
	MinSize   float64 // 最小开仓金额（0 表示使用线性插值）
	MaxSize   float64 // 最大开仓金额（用于线性插值）
}

var (
	// 配置常量
	absoluteMinimum = 12.0 // 交易所绝对最小值 (10 USDT + 20% 安全边际)
	standardBTCETH  = 60.0 // 标准 BTC/ETH 最小值 (因价格高和精度限制)

	// BTC/ETH 动态调整规则（按账户规模分层）
	btcEthSizeRules = []positionSizeConfig{
		{MinEquity: 0, MinSize: absoluteMinimum, MaxSize: absoluteMinimum}, // 小账户(<20U): 12 USDT
		{MinEquity: 20, MinSize: absoluteMinimum, MaxSize: standardBTCETH}, // 中型账户(20-100U): 线性插值
		{MinEquity: 100, MinSize: standardBTCETH, MaxSize: standardBTCETH}, // 大账户(≥100U): 60 USDT
	}

	// 山寨币规则（始终使用绝对最小值）
	altcoinSizeRules = []positionSizeConfig{
		{MinEquity: 0, MinSize: absoluteMinimum, MaxSize: absoluteMinimum},
	}

	// 币种规则映射表（易于扩展，添加新币种只需在此添加一行）
	symbolSizeRules = map[string][]positionSizeConfig{
		"BTCUSDT": btcEthSizeRules,
		"ETHUSDT": btcEthSizeRules,
	}
)

// calculateMinPositionSize 根据账户净值和币种动态计算最小开仓金额
func calculateMinPositionSize(symbol string, accountEquity float64) float64 {
	// 从配置映射表中获取币种规则
	rules, exists := symbolSizeRules[symbol]
	if !exists {
		// 未配置的币种使用山寨币规则（默认绝对最小值）
		rules = altcoinSizeRules
	}

	// 根据规则表动态计算
	for i, rule := range rules {
		// 找到账户所属的规模区间
		if i == len(rules)-1 || accountEquity < rules[i+1].MinEquity {
			// 如果 MinSize == MaxSize，直接返回固定值
			if rule.MinSize == rule.MaxSize {
				return rule.MinSize
			}
			// 否则使用线性插值
			nextRule := rules[i+1]
			equityRange := nextRule.MinEquity - rule.MinEquity
			sizeRange := rule.MaxSize - rule.MinSize
			return rule.MinSize + sizeRange*(accountEquity-rule.MinEquity)/equityRange
		}
	}

	// 默认返回绝对最小值（理论上不会执行到这里）
	return absoluteMinimum
}

// validateDecision 验证单个决策的有效性（增强版）
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, currentPositions []PositionInfo) error {
	return validateDecisionWithMarketData(d, accountEquity, btcEthLeverage, altcoinLeverage, currentPositions, nil)
}

// validateDecisionWithMarketData 验证单个决策的有效性（支持模拟数据)
func validateDecisionWithMarketData(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, currentPositions []PositionInfo, mockMarketData *market.Data) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 使用增强版验证器进行详细检查
		validator := NewEnhancedValidator(accountEquity, btcEthLeverage, altcoinLeverage, currentPositions)

		// 获取市场数据用于验证
		var marketData *market.Data
		var err error

		if mockMarketData != nil {
			// 使用提供的模拟数据
			marketData = mockMarketData
		} else {
			// 尝试获取真实市场数据
			marketData, err = market.Get(d.Symbol, []string{"15m", "1h", "4h"})
			if err != nil {
				return fmt.Errorf("无法获取 %s 的市场数据: %w", d.Symbol, err)
			}
		}
		validator.MarketData[d.Symbol] = marketData

		// ==================== V6.0 新增：硬性物理过滤器 ====================

		// 1. 同向持仓限制 (已禁用 - 中长线策略允许多币种同向分散风险)
		// 原限制：已有空单则禁止再开任何空单，已有多单则禁止再开任何多单
		// 禁用理由：
		//   - 中长线策略基于大周期趋势（日线/4H 共振），多币种同向是合理的分散策略
		//   - 已有其他风控保护：持仓数量上限3个、单笔风险2%、独立止损(ATR*3)
		//   - 允许 BTC空 + ETH空 + SOL空，只要每个都符合趋势判断
		// 保留风控：同币种重复持仓检查（防止 BTCUSDT 重复开空）
		if false { // 使用 false 禁用此逻辑
			if d.Action == "open_short" || d.Action == "open_long" {
				for _, pos := range currentPositions {
					if d.Action == "open_short" && pos.Side == "short" {
						return fmt.Errorf("风控拦截: 已持有空单 (%s)，禁止多币种同向赌博", pos.Symbol)
					}
					if d.Action == "open_long" && pos.Side == "long" {
						return fmt.Errorf("风控拦截: 已持有多单 (%s)，禁止多币种同向赌博", pos.Symbol)
					}
				}
			}
		}

		// 2. RSI 硬性熔断 (中长线策略: 已禁用)
		// 理由: 中长线策略基于大周期趋势（日线/4H EMA 共振），短期 RSI 超买超卖是正常现象
		// 例如：下跌趋势中，RSI 可能长期处于超卖区（< 30），这是趋势强度的体现而非反转信号
		// 风控依赖: 止损设置(ATR*3) + 同向持仓限制 + 乖离率保护
		if false && marketData != nil { // 使用 false 禁用此逻辑，保留代码以便未来恢复
			rsi := marketData.CurrentRSI7 // 使用 7周期 RSI 更灵敏

			if d.Action == "open_short" {
				if rsi < 30 {
					return fmt.Errorf("风控拦截: RSI (%.2f) 处于超卖区，禁止追空", rsi)
				}
			}
			if d.Action == "open_long" {
				if rsi > 70 {
					return fmt.Errorf("风控拦截: RSI (%.2f) 处于超买区，禁止追高", rsi)
				}
			}

			// 3. 乖离率 (EMA Deviation) 保护
			// 防止在暴跌后追单
			if marketData.MidTermSeries15m != nil && len(marketData.MidTermSeries15m.EMA20Values) > 0 {
				ema20 := marketData.MidTermSeries15m.EMA20Values[len(marketData.MidTermSeries15m.EMA20Values)-1]
				price := marketData.CurrentPrice

				// 计算偏离度
				deviation := (price - ema20) / ema20

				// 开空时，如果价格已经比 EMA 低了 1% 以上，说明跌太急了
				if d.Action == "open_short" && deviation < -0.01 {
					return fmt.Errorf("风控拦截: 乖离率过大 (%.2f%%)，价格远离均线，禁止追空", deviation*100)
				}
				// 开多时，如果价格已经比 EMA 高了 1% 以上
				if d.Action == "open_long" && deviation > 0.01 {
					return fmt.Errorf("风控拦截: 乖离率过大 (%.2f%%)，价格远离均线，禁止追多", deviation*100)
				}
			}
		}

		// ================================================================

		// 执行增强验证
		result := validator.ValidateDecision(d)

		// 记录验证详情
		if len(result.Warnings) > 0 {
			log.Printf("⚠️ %s 验证警告: %v", d.Symbol, result.Warnings)
		}

		// 如果有致命错误，返回详细错误信息
		if !result.IsValid {
			errorMsg := fmt.Sprintf("决策验证失败 (风险等级: %s, 风险比例: %.2f%%): ",
				result.RiskLevel, result.RiskPercent)
			for i, err := range result.Errors {
				if i > 0 {
					errorMsg += "; "
				}
				errorMsg += err
			}
			return fmt.Errorf("%s", errorMsg)
		}

		// 记录风险控制信息
		log.Printf("✅ %s 风险控制通过: 风险等级=%s, 风险比例=%.2f%%, 杠杆=%dx, 仓位=$%.2f",
			d.Symbol, result.RiskLevel, result.RiskPercent, d.Leverage, d.PositionSizeUSD)
	}

	// 动态调整止损验证
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("新止损价格必须大于0: %.2f", d.NewStopLoss)
		}
	}

	// 动态调整止盈验证
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("新止盈价格必须大于0: %.2f", d.NewTakeProfit)
		}
	}

	// 部分平仓验证
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("partial_close ClosePercentage必须在1-100之间，当前值: %.2f", d.ClosePercentage)
		}
		if d.ClosePercentage < 5.0 {
			return fmt.Errorf("partial_close ClosePercentage过小(%.1f%%)，建议≥5%%以确保有足够的平仓价值", d.ClosePercentage)
		}
	}

	return nil
}
