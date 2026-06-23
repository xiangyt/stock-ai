package monitor

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

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

	// 计算窗口首尾相对昨收的涨跌幅差值
	amplitude, _ := calcWindowAmplitude(data.Minutes, params.Minutes, data.PreClose)
	if amplitude == 0 {
		return nil
	}

	var alerts []Alert
	if params.UpEnabled && amplitude >= params.AmplitudePct {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeRapidMove),
			SubType:   "rapid_up",
			Label:     "急拉",
			ChangePct: amplitude,
			Minutes:   params.Minutes,
			Amplitude: amplitude,
		})
	}
	if params.DownEnabled && amplitude <= -params.AmplitudePct {
		alerts = append(alerts, Alert{
			RuleType:  string(model.RuleTypeRapidMove),
			SubType:   "rapid_down",
			Label:     "急跌",
			ChangePct: amplitude,
			Minutes:   params.Minutes,
			Amplitude: amplitude,
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

	// 10:00 前数据量不足，量比严重虚高，跳过检测
	if len(data.Minutes) > 0 {
		lastTime := data.Minutes[len(data.Minutes)-1].Time
		if parseMinute(lastTime) < 10*60 {
			return nil
		}
	}

	// 当日累计成交量（股）
	todayVol := float64(data.Volume)
	if todayVol <= 0 {
		return nil
	}

	// 当日累计开市分钟数：用分时 bar 数量近似，上限 240（全天交易分钟数）
	elapsedMinutes := len(data.Minutes)
	if elapsedMinutes <= 0 {
		return nil
	}
	if elapsedMinutes > 240 {
		elapsedMinutes = 240
	}

	// 量比 = 现成交总量 ÷ (过去5个交易日平均每分钟成交量 × 当日累计开市分钟数)
	// 过去5个交易日平均每分钟成交量 = hist5DayVol / (5 × 240)
	// 化简: ratio = todayVol × 5 × 240 / (hist5DayVol × elapsedMinutes)
	todayDate, _ := strconv.Atoi(strings.ReplaceAll(data.Date, "-", ""))
	hist5DayVol, err := db.GetVolumeSum5DayHistorical(data.Code, todayDate)
	if err != nil || hist5DayVol <= 0 {
		return nil
	}
	ratio := todayVol * float64(5*240) / (float64(hist5DayVol) * float64(elapsedMinutes))

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

	if data.Depth == nil {
		return nil
	}

	pct := data.ChangePct
	d := data.Depth

	// 涨停：涨幅 ≥ 9.8% 且买一有量卖一无量（确认封涨停）
	if params.UpEnabled && pct >= 9.8 && d.Bid1Volume > 0 && d.Ask1Volume == 0 {
		sealLots := int(d.Bid1Volume / 100) // 股 → 手
		if sealLots < params.MinLots {
			return []Alert{{
				RuleType:  string(model.RuleTypeSealBoard),
				SubType:   "bid_low",
				Label:     "涨停买一薄弱",
				ChangePct: pct,
				MinLots:   params.MinLots,
			}}
		}
	}

	// 跌停：跌幅 ≤ -9.8% 且卖一有量买一无量（确认封跌停）
	if params.DownEnabled && pct <= -9.8 && d.Ask1Volume > 0 && d.Bid1Volume == 0 {
		sealLots := int(d.Ask1Volume / 100) // 股 → 手
		if sealLots < params.MinLots {
			return []Alert{{
				RuleType:  string(model.RuleTypeSealBoard),
				SubType:   "ask_low",
				Label:     "跌停卖一薄弱",
				ChangePct: pct,
				MinLots:   params.MinLots,
			}}
		}
	}

	return nil
}

// calcWindowAmplitude 根据窗口首尾 bar 计算急拉急跌振幅。
//
// 取窗口起始 bar（N 交易分钟前）和最新 bar 的价格，
// 分别计算相对昨收的涨跌幅，差值即为窗口振幅。
//
//	headPct = (起始价 - preClose) / preClose * 100
//	tailPct = (最新价 - preClose) / preClose * 100
//	amplitude = tailPct - headPct
//	dir: >0 急拉, <0 急跌
//
// 若窗口内无有效分时数据，返回 0。
func calcWindowAmplitude(bars []model.MinuteBar, windowMinutes int, preClose float64) (amplitude float64, dir int) {
	if len(bars) == 0 || preClose <= 0 {
		return 0, 0
	}

	tail := bars[len(bars)-1]
	if tail.Time == "" {
		return 0, 0
	}

	tailTradeMin := tradingMinute(tail.Time)
	headTradeMin := tailTradeMin - windowMinutes
	if headTradeMin < 0 {
		headTradeMin = 0
	}

	// 向前扫描找窗口起始 bar
	var headPrice float64
	for i := len(bars) - 1; i >= 0; i-- {
		tm := tradingMinute(bars[i].Time)
		if tm <= headTradeMin {
			headPrice = bars[i].Price
			break
		}
	}
	if headPrice <= 0 {
		headPrice = bars[0].Price
	}
	if headPrice <= 0 {
		return 0, 0
	}

	headPct := (headPrice - preClose) / preClose * 100
	tailPct := (tail.Price - preClose) / preClose * 100
	amplitude = tailPct - headPct

	switch {
	case amplitude > 0:
		dir = 1
	case amplitude < 0:
		dir = -1
	}
	return
}

// parseMinute 将 "HH:MM" 转为绝对分钟数（0:00 开始）。
func parseMinute(t string) int {
	if len(t) < 5 {
		return 0
	}
	h := int(t[0]-'0')*10 + int(t[1]-'0')
	m := int(t[3]-'0')*10 + int(t[4]-'0')
	return h*60 + m
}

// tradingMinute 将 "HH:MM" 转为交易分钟数（9:30 开盘 = 0，午休 11:30-13:00 跳过）。
//
//	9:30 → 0,  11:30 → 120,  13:00 → 120,  15:00 → 240
func tradingMinute(t string) int {
	m := parseMinute(t)
	if m <= 0 {
		return 0
	}
	// 上午: 9:30(570) ~ 11:30(690)，从 570 起算
	if m <= 11*60+30 {
		return m - 9*60 - 30
	}
	// 下午: 13:00(780) ~ 15:00(900)，扣除 90 分钟午休
	return m - 9*60 - 30 - 90
}
