package monitor

import (
	"encoding/json"
	"log"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ============================================================================
//  Alert — 告警结果
// ============================================================================

// Alert 单条告警结果
type Alert struct {
	RuleType  string  // 规则类型: "daily_change" / "rapid_move" / "volume_ratio" / "seal_board"
	SubType   string  // 子类型: "surge_big" / "rapid_up" / "high_volume" / "bid_low" 等
	Label     string  // 中文标签，用于推送消息
	ChangePct float64 // 涨跌幅（如有）
	// 规则特有字段（用于模板渲染，未使用字段为 Go 零值）
	Minutes     int     // rapid_move: 时间窗口(分钟)
	Amplitude   float64 // rapid_move: 幅度阈值
	VolumeRatio float64 // volume_ratio: 计算出的实际量比
	MinLots     int     // seal_board: 封单阈值(手)
}

// ============================================================================
//  AlertChecker — 规则检查器（按 rule.type 动态派发）
// ============================================================================

type AlertChecker struct{}

func NewAlertChecker() *AlertChecker {
	return &AlertChecker{}
}

// Check 根据规则检查行情数据，返回触发的告警列表
func (c *AlertChecker) Check(rule model.MonitorRule, data *model.QuoteData) []Alert {
	if data == nil {
		return nil
	}

	switch rule.Type {
	case model.RuleTypeDailyChange:
		return c.checkDailyChange(rule, data)
	case model.RuleTypeRapidMove:
		return c.checkRapidMove(rule, data)
	case model.RuleTypeVolumeRatio:
		return c.checkVolumeRatio(rule, data)
	case model.RuleTypeSealBoard:
		return c.checkSealBoard(rule, data)
	default:
		log.Printf("[Monitor] 未知的规则类型: %s", rule.Type)
		return nil
	}
}

// ============================================================================
//  checkDailyChange — 当日涨幅监控（6档）
// ============================================================================

func (c *AlertChecker) checkDailyChange(rule model.MonitorRule, data *model.QuoteData) []Alert {
	var params model.DailyChangeParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		log.Printf("[Monitor] 解析当日涨幅参数失败: %v", err)
		return nil
	}

	pct := data.ChangePct
	var alerts []Alert

	// 涨停
	if params.LimitUpEnabled && params.LimitUp > 0 && pct >= params.LimitUp {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "limit_up",
			Label:     "涨停",
			ChangePct: pct,
		})
		return alerts
	}

	// 跌停
	if params.LimitDownEnabled && params.LimitDown < 0 && pct <= params.LimitDown {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "limit_down",
			Label:     "跌停",
			ChangePct: pct,
		})
		return alerts
	}

	// 大涨
	if params.SurgeBigEnabled && params.SurgeBig > 0 && pct >= params.SurgeBig {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "surge_big",
			Label:     "大涨",
			ChangePct: pct,
		})
	} else if params.SurgeSmallEnabled && params.SurgeSmall > 0 && pct >= params.SurgeSmall {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "surge_small",
			Label:     "小涨",
			ChangePct: pct,
		})
	}

	// 大跌
	if params.DropBigEnabled && params.DropBig < 0 && pct <= params.DropBig {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "drop_big",
			Label:     "大跌",
			ChangePct: pct,
		})
	} else if params.DropSmallEnabled && params.DropSmall < 0 && pct <= params.DropSmall {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeDailyChange),
			SubType:   "drop_small",
			Label:     "小跌",
			ChangePct: pct,
		})
	}

	return alerts
}

// ============================================================================
//  checkRapidMove — 急拉急跌监控
// ============================================================================

func (c *AlertChecker) checkRapidMove(rule model.MonitorRule, data *model.QuoteData) []Alert {
	var params model.RapidMoveParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		log.Printf("[Monitor] 解析急拉急跌参数失败: %v", err)
		return nil
	}

	pct := data.ChangePct
	// 若有分时数据，计算指定窗口内的涨跌幅
	if len(data.Minutes) > 0 {
		pct = calcWindowPct(data.Minutes, params.Minutes, data.Price)
	}
	var alerts []Alert

	if params.UpEnabled && pct >= params.AmplitudePct {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeRapidMove),
			SubType:   "rapid_up",
			Label:     "急拉",
			ChangePct: pct,
			Minutes:   params.Minutes,
			Amplitude: params.AmplitudePct,
		})
	}
	if params.DownEnabled && pct <= -params.AmplitudePct {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeRapidMove),
			SubType:   "rapid_down",
			Label:     "急跌",
			ChangePct: pct,
			Minutes:   params.Minutes,
			Amplitude: params.AmplitudePct,
		})
	}
	return alerts
}

// ============================================================================
//  checkVolumeRatio — 量比异动监控
// ============================================================================

func (c *AlertChecker) checkVolumeRatio(rule model.MonitorRule, data *model.QuoteData) []Alert {
	var params model.VolumeRatioParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		log.Printf("[Monitor] 解析量比参数失败: %v", err)
		return nil
	}

	todayVol := float64(data.Volume)
	if todayVol <= 0 {
		return nil
	}

	// 查询近5日日均成交量（从 DB K线表）
	avgVol, err := db.GetAvgVolume5Day(data.Code)
	if err != nil || avgVol <= 0 {
		return nil // 数据不足，跳过
	}

	ratio := todayVol / avgVol
	if ratio >= params.MinRatio {
		return []Alert{{
			RuleType:    string(model.RuleTypeVolumeRatio),
			SubType:     "high_volume",
			Label:       "量比异动",
			ChangePct:   data.ChangePct,
			VolumeRatio: ratio,
		}}
	}

	return nil
}

// ============================================================================
//  checkSealBoard — 涨跌停封单数量监控
// ============================================================================

func (c *AlertChecker) checkSealBoard(rule model.MonitorRule, data *model.QuoteData) []Alert {
	var params model.SealBoardParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		log.Printf("[Monitor] 解析封单参数失败: %v", err)
		return nil
	}

	pct := data.ChangePct

	// TODO: quotecache / adapter 当前不提供封单数据（Level 2 行情）
	// 封单数据接入后实现：
	//
	// 涨停时看买一: if pct >= 9.8 && sealData.BidLots < params.MinLots {
	//     return []Alert{{RuleType: "seal_board", SubType: "bid_low", Label: "涨停买一薄弱"}}
	// }
	// 跌停时看卖一: if pct <= -9.8 && sealData.AskLots < params.MinLots {
	//     return []Alert{{RuleType: "seal_board", SubType: "ask_low", Label: "跌停卖一薄弱"}}
	// }
	_ = params
	_ = pct

	return nil
}

// calcWindowPct 根据分时数据计算指定窗口内的涨跌幅
// bars 按时间升序，windowMinutes 为窗口大小，currentPrice 为当前价
// 从最新 bar 向前查找，找到 >= windowMinutes 之前的 bar，计算涨幅
// 若无分时数据，调用方应降级使用全天涨跌幅
func calcWindowPct(bars []model.MinuteBar, windowMinutes int, currentPrice float64) float64 {
	if len(bars) == 0 || currentPrice <= 0 {
		return 0
	}
	// 取最新 bar 作为基准时间
	latest := bars[len(bars)-1]
	if latest.Time == "" {
		return 0
	}
	// 解析最新时间，计算窗口起始时间
	latestMin := parseMinute(latest.Time)
	startMin := latestMin - windowMinutes
	if startMin < 0 {
		startMin = 0
	}
	// 向前扫描找到最早 >= startMin 的 bar
	var basePrice float64
	for i := len(bars) - 1; i >= 0; i-- {
		m := parseMinute(bars[i].Time)
		if m <= startMin {
			basePrice = bars[i].Price
			break
		}
	}
	if basePrice <= 0 {
		// 窗口内无足够数据，取最早 bar
		basePrice = bars[0].Price
	}
	if basePrice <= 0 {
		return 0
	}
	return (currentPrice - basePrice) / basePrice * 100
}

// parseMinute 将 "HH:MM" 转为分钟数（9:00 开始）
func parseMinute(t string) int {
	if len(t) < 5 {
		return 0
	}
	h := int(t[0]-'0')*10 + int(t[1]-'0')
	m := int(t[3]-'0')*10 + int(t[4]-'0')
	return h*60 + m
}
