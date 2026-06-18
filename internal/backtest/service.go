package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	stocksource "stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/utils"

	"stock-ai/internal/backtest/exit"
)

// Service 回测异步执行管理器
type Service struct {
	dao    *DAO
	engine *indicator.Engine
	mu     sync.Mutex
	running map[uint64]context.CancelFunc
}

// NewService 创建回测服务实例
func NewService(e *indicator.Engine) *Service {
	return &Service{
		dao:     NewDAO(),
		engine:  e,
		running: make(map[uint64]context.CancelFunc),
	}
}

// ============================================================================
//  Initiate — 启动回测（异步）
// ============================================================================

// Initiate 创建回测运行记录并异步启动引擎。
// 返回 runID 供前端轮询状态。
func (s *Service) Initiate(ctx context.Context, strategyID uint64, stockPool []string,
	startDate, endDate string, initialCapital float64,
	exitOverride *model.ExitRules, positionOverride *model.PositionRules) (uint64, error) {

	// 加载策略
	strategy, err := db.GetStrategyByID(uint(strategyID))
	if err != nil {
		return 0, fmt.Errorf("load strategy: %w", err)
	}

	// 解析卖出规则与仓位规则
	var exitRules model.ExitRules
	if strategy.ExitRules != "" {
		_ = json.Unmarshal([]byte(strategy.ExitRules), &exitRules)
	}
	var positionRules model.PositionRules
	if strategy.PositionRules != "" {
		_ = json.Unmarshal([]byte(strategy.PositionRules), &positionRules)
	}

	// 应用 override（如果前端传了则覆盖策略默认规则）
	if exitOverride != nil {
		exitRules = *exitOverride
	}
	if positionOverride != nil {
		positionRules = *positionOverride
	}

	// 加载用户手续费配置
	commissionRate := 2.5 // 默认万分之2.5
	minCommission := true // 默认不免五
	if user, err := db.GetUserByID(strategy.UID); err == nil {
		if user.CommissionRate > 0 {
			commissionRate = user.CommissionRate
		}
		minCommission = user.MinCommission
	}

	poolJSON := mustMarshal(stockPool)
	exitJSON := mustMarshal(exitRules)
	posJSON := mustMarshal(positionRules)

	run := &BacktestRun{
		StrategyID:     strategyID,
		StockPool:      poolJSON,
		StartDate:      startDate,
		EndDate:        endDate,
		InitialCapital: initialCapital,
		ExitRules:      exitJSON,
		PositionRules:  posJSON,
		Status:         "pending",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.dao.CreateRun(run); err != nil {
		return 0, fmt.Errorf("create run: %w", err)
	}

	// 异步启动
	go s.run(run.ID, strategy, stockPool, startDate, endDate, initialCapital, &exitRules, &positionRules, commissionRate, minCommission)

	return run.ID, nil
}

// ============================================================================
//  runContext — 回测运行时上下文
// ============================================================================

// runContext 回测引擎单次运行的上下文
type runContext struct {
	runID          uint64
	initialCapital float64
	cash           float64
	holdings       map[string]*HoldingPosition // stockCode -> HoldingPosition
	dailySnapshots []DailySnapshot
	trades         []BacktestTrade
	startDate      string
	endDate        string
	commissionRate float64 // 手续费率(万分之x)，默认 2.5
	minCommission  bool    // 是否不免五: false=免五 true=不免五
}

// ============================================================================
//  run — 回测主循环（goroutine）
// ============================================================================

func (s *Service) run(runID uint64, strategy *model.Strategy, stockPool []string,
	startDate, endDate string, initialCapital float64,
	exitRules *model.ExitRules, positionRules *model.PositionRules,
	commissionRate float64, minCommission bool) {

	defer func() {
		if r := recover(); r != nil {
			s.failRun(runID, fmt.Sprintf("panic: %v", r))
		}
	}()

	// 1. 计算交易日列表
	tradingDays, err := getTradingDays(startDate, endDate)
	if err != nil {
		s.failRun(runID, "计算交易日失败: "+err.Error())
		return
	}
	if len(tradingDays) == 0 {
		s.failRun(runID, "回测区间无交易日")
		return
	}

	// 2. 解析策略条件
	sc, err := parseStrategyRules(strategy)
	if err != nil {
		s.failRun(runID, "解析策略条件失败: "+err.Error())
		return
	}

	// 2.5 如果 stock_pool 为空，使用全市场股票
	if len(stockPool) == 0 {
		allStocks := db.LoadAllStockCodes()
		stockPool = make([]string, len(allStocks))
		for i, st := range allStocks {
			stockPool[i] = st.Code
		}
	}

	// 3. 更新状态为 running
	s.dao.UpdateRun(&BacktestRun{ID: runID, Status: "running", UpdatedAt: time.Now()})

	// 4. 初始化运行时上下文
	rc := &runContext{
		runID:          runID,
		initialCapital: initialCapital,
		cash:           initialCapital,
		holdings:       make(map[string]*HoldingPosition),
		dailySnapshots: make([]DailySnapshot, 0, len(tradingDays)),
		trades:         make([]BacktestTrade, 0),
		startDate:      startDate,
		endDate:        endDate,
		commissionRate: commissionRate,
		minCommission:  minCommission,
	}

	// 5. 构建卖出规则检查链（可插拔架构）
	exitChain, err := BuildExitChain(exitRules.Rules, exitRules.SlippagePct)
	if err != nil {
		s.failRun(runID, "构建卖出规则链失败: "+err.Error())
		return
	}

	// 5.5 为信号退出规则注入评估器
	for _, c := range exitChain {
		if sc, ok := c.(*exit.SignalExitChecker); ok {
			sc.SetEvaluator(func(signalID, operator string, params map[string]any, code string, dateInt int) bool {
				stock, err := db.FindStockByCode(code)
				if err != nil {
					return false
				}
				src := stocksource.NewDBStock(&stock, dateInt)
				cfg := &indicator.SignalConfig{
					SignalID: signalID,
					Operator: indicator.CompareOperator(operator),
					Params:   params,
				}
				results := s.engine.Execute([]indicator.StockSource{src}, []*indicator.SignalConfig{cfg}, 1)
				return len(results) > 0 && results[0].Result == indicator.ResultPassed
			})
		}
	}

	// 6. 逐日执行回测
	prevEquity := initialCapital
	total := len(tradingDays)

	for i, date := range tradingDays {
		// 加载当日行情
		bars := s.loadDayBars(date, stockPool, rc.holdings)
		if len(bars) == 0 {
			// 无行情数据的交易日（跳过，但记录快照）
			snap := DailySnapshot{
				RunID:            runID,
				SnapDate:         date,
				TotalEquity:      prevEquity,
				Cash:             rc.cash,
				MarketValue:      prevEquity - rc.cash,
				PositionCount:    len(rc.holdings),
				CumulativeReturn: ptrFloat((prevEquity - initialCapital) / initialCapital * 100),
				CreatedAt:        time.Now(),
			}
			s.dao.CreateSnapshot(&snap)
			rc.dailySnapshots = append(rc.dailySnapshots, snap)
			continue
		}

		// --- 步骤1: 检查卖出 ---
		for code, pos := range rc.holdings {
			bar, ok := bars[code]
			if !ok {
				continue
			}
			// 更新所有实现 DailyUpdateHook 的 checker 状态
			for _, c := range exitChain {
				if h, ok := c.(DailyUpdateHook); ok {
					h.OnDailyUpdate(pos, bar)
				}
			}
			// 按优先级检查（exitChain 已排序）
			for _, c := range exitChain {
				if decision := c.Check(pos, bar); decision != nil {
					if decision.Quantity > 0 {
						s.executePartialSell(rc, code, pos, bar, decision, date)
					} else {
						s.executeSell(rc, code, pos, bar, decision, date)
					}
					break // 触发一个就卖出，不再检查低优先级规则
				}
			}
		}

		// --- 步骤2: 检查买入 ---
		maxPos := positionRules.MaxPositions
		shouldBuy := maxPos <= 0 || len(rc.holdings) < maxPos

		if shouldBuy {
			// 收集未持仓的候选股票
			candidates := make([]string, 0)
			for _, code := range stockPool {
				if _, held := rc.holdings[code]; !held {
					candidates = append(candidates, code)
				}
			}
			if len(candidates) > 0 {
				s.checkBuySignals(rc, candidates, bars, sc, positionRules, exitChain, date)
			}
		}

		// --- 步骤3: 记录快照 ---
		marketValue := 0.0
		for _, pos := range rc.holdings {
			if bar, ok := bars[pos.StockCode]; ok {
				marketValue += float64(pos.Quantity) * bar.Close
			}
		}
		totalEquity := rc.cash + marketValue

		dayReturn := 0.0
		if prevEquity > 0 {
			dayReturn = (totalEquity - prevEquity) / prevEquity * 100
		}

		cumReturn := (totalEquity - initialCapital) / initialCapital * 100

		snap := DailySnapshot{
			RunID:            runID,
			SnapDate:         date,
			TotalEquity:      totalEquity,
			Cash:             rc.cash,
			MarketValue:      marketValue,
			PositionCount:    len(rc.holdings),
			DailyReturn:      ptrFloat(dayReturn),
			CumulativeReturn: ptrFloat(cumReturn),
			CreatedAt:        time.Now(),
		}
		s.dao.CreateSnapshot(&snap)
		rc.dailySnapshots = append(rc.dailySnapshots, snap)
		prevEquity = totalEquity

		// --- 进度更新 ---
		progressPct := (i + 1) * 100 / total
		if progressPct%10 == 0 || i == total-1 {
			s.dao.UpdateRun(&BacktestRun{ID: runID, ProgressPct: progressPct, UpdatedAt: time.Now()})
		}
	}

	// 7. 强制清仓（最后一天按收盘价清掉所有持仓）
	lastDate := tradingDays[len(tradingDays)-1]
	lastBars := s.loadDayBars(lastDate, stockPool, rc.holdings)
	for code, pos := range rc.holdings {
		if bar, ok := lastBars[code]; ok {
			s.executeForceClose(rc, code, pos, bar, lastDate)
		}
	}

	// 8. 计算绩效指标并持久化
	s.finalize(runID, rc, len(tradingDays))
}

// ============================================================================
//  子流程
// ============================================================================

// loadDayBars 从数据库 daily_kline 表加载指定日期的日K数据
func (s *Service) loadDayBars(date string, stockPool []string, holdings map[string]*HoldingPosition) map[string]DayBar {
	bars := make(map[string]DayBar)

	// 收集所有需要的股票代码
	allCodes := make(map[string]bool)
	for _, code := range stockPool {
		allCodes[code] = true
	}
	for code := range holdings {
		allCodes[code] = true
	}
	if len(allCodes) == 0 {
		return bars
	}

	// 日期字符串 → YYYYMMDD int
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return bars
	}
	tradeDate := t.Year()*10000 + int(t.Month())*100 + t.Day()

	// 收集代码列表
	codes := make([]string, 0, len(allCodes))
	for code := range allCodes {
		codes = append(codes, code)
	}

	// 一次查询：按 (stock_code, trade_date) 批量查
	var klines []model.DailyKline
	if err := db.GetDB().Where("stock_code IN ? AND trade_date = ?", codes, tradeDate).Find(&klines).Error; err != nil {
		return bars
	}

	for _, k := range klines {
		// DailyKline 价格单位是"分"，转为"元"
		bars[k.StockCode] = DayBar{
			Date:  date,
			Open:  float64(k.Open) / 100,
			High:  float64(k.High) / 100,
			Low:   float64(k.Low) / 100,
			Close: float64(k.Close) / 100,
		}
	}
	return bars
}

// executeSell 执行卖出操作
func (s *Service) executeSell(rc *runContext, code string, pos *HoldingPosition, bar DayBar, decision *ExitDecision, date string) {
	sellPrice := decision.Price
	sellAmount := sellPrice * float64(pos.Quantity)

	// 手续费（佣金 + 印花税）
	commission := calcCommission(sellAmount, rc.commissionRate, rc.minCommission)
	stampTax := calcStampTax(sellAmount)

	// 更新现金
	rc.cash += sellAmount - commission - stampTax

	// 计算盈亏
	entryAmount := pos.EntryPrice * float64(pos.Quantity)
	profitLoss := sellAmount - entryAmount - commission - stampTax
	profitLossPct := profitLoss / entryAmount * 100

	exitReason := decision.Reason
	preExitPrice := sellPrice

	rc.trades = append(rc.trades, BacktestTrade{
		RunID:         rc.runID,
		StockCode:     code,
		TradeType:     2, // 卖出
		Quantity:      pos.Quantity,
		Price:         sellPrice,
		Amount:        sellAmount,
		Commission:    commission,
		StampTax:      stampTax,
		TradeDate:     NewDateOnly(date),
		ExitReason:    &exitReason,
		PreExitPrice:  &preExitPrice,
		ProfitLoss:    &profitLoss,
		ProfitLossPct: &profitLossPct,
		CreatedAt:     time.Now(),
	})

	delete(rc.holdings, code)
}

// executeForceClose 强制清仓（回测最后一天）
func (s *Service) executeForceClose(rc *runContext, code string, pos *HoldingPosition, bar DayBar, date string) {
	closePrice := bar.Close
	amount := closePrice * float64(pos.Quantity)

	commission := calcCommission(amount, rc.commissionRate, rc.minCommission)
	stampTax := calcStampTax(amount)

	rc.cash += amount - commission - stampTax

	entryAmount := pos.EntryPrice * float64(pos.Quantity)
	profitLoss := amount - entryAmount - commission - stampTax
	profitLossPct := profitLoss / entryAmount * 100

	reason := "force_close"

	rc.trades = append(rc.trades, BacktestTrade{
		RunID:         rc.runID,
		StockCode:     code,
		TradeType:     2,
		Quantity:      pos.Quantity,
		Price:         closePrice,
		Amount:        amount,
		Commission:    commission,
		StampTax:      stampTax,
		TradeDate:     NewDateOnly(date),
		ExitReason:    &reason,
		ProfitLoss:    &profitLoss,
		ProfitLossPct: &profitLossPct,
		CreatedAt:     time.Now(),
	})

	delete(rc.holdings, code)
}

// checkBuySignals 检查买入信号并执行买入
func (s *Service) checkBuySignals(rc *runContext, candidates []string, bars map[string]DayBar,
	sc *strategyConditions, posRules *model.PositionRules, exitChain []ExitChecker, date string) {

	if len(sc.Conditions) == 0 {
		return
	}

	// 1. 只对有当日行情数据的候选股做信号评估
	tradeDate, err := utils.ParseDateToTradeDate(date)
	if err != nil {
		return
	}

	// 过滤出有行情数据的候选股
	validCandidates := make([]string, 0, len(candidates))
	for _, code := range candidates {
		if _, ok := bars[code]; ok {
			validCandidates = append(validCandidates, code)
		}
	}
	if len(validCandidates) == 0 {
		return
	}

	// 2. 构建 StockSource 列表（用于 Engine.Execute 信号评估）
	sources := make([]indicator.StockSource, 0, len(validCandidates))
	sourceMap := make(map[string]indicator.StockSource, len(validCandidates))
	for _, code := range validCandidates {
		stock, err := db.FindStockByCode(code)
		if err != nil {
			continue
		}
		src := stocksource.NewDBStock(&stock, tradeDate)
		sources = append(sources, src)
		sourceMap[code] = src
	}

	if len(sources) == 0 {
		return
	}

	// 3. 调用 Engine.Execute 做信号评估
	results := s.engine.Execute(sources, sc.Conditions, 10) // 并发度 10
	passed := make(map[string]bool, len(results))
	for _, r := range results {
		if r.Result == indicator.ResultPassed {
			passed[r.Code] = true
		}
	}

	// 4. 收集通过信号的候选股信息，构建 Allocator 请求
	candidateInfos := make([]CandidateInfo, 0, len(passed))
	for _, code := range validCandidates {
		if !passed[code] {
			continue
		}
		bar := bars[code]
		// 计算波动率（从 StockSource 获取近期 K 线）
		volatility := calcVolatility(sourceMap[code])
		candidateInfos = append(candidateInfos, CandidateInfo{
			Code:        code,
			Price:       bar.Close,
			SignalScore: 1.0, // 默认评分（未来可从信号评估结果获取）
			Volatility:  volatility,
		})
	}

	if len(candidateInfos) == 0 {
		return
	}

	// 5. 计算总权益
	totalEquity := rc.cash
	for _, p := range rc.holdings {
		if b, ok2 := bars[p.StockCode]; ok2 {
			totalEquity += float64(p.Quantity) * b.Close
		}
	}

	// 6. 创建 Allocator 并分配资金
	allocator, err := CreateAllocator(posRules.Allocation)
	if err != nil {
		return
	}

	allocReq := &AllocRequest{
		Cash:            rc.cash,
		TotalEquity:     totalEquity,
		MaxPositions:    posRules.MaxPositions,
		CurrentHoldings: len(rc.holdings),
		MaxSinglePct:    posRules.MaxSinglePct,
		CashBufferPct:   posRules.CashBufferPct,
		Candidates:      candidateInfos,
	}
	allocResult := allocator.Allocate(allocReq)

	// 7. 按分配结果执行买入
	for _, order := range allocResult.Orders {
		bar, ok := bars[order.Code]
		if !ok {
			continue
		}

		buyPrice := bar.Close
		buyQty := order.Quantity
		buyAmount := buyPrice * float64(buyQty)

		commission := calcCommission(buyAmount, rc.commissionRate, rc.minCommission)
		totalCost := buyAmount + commission

		if rc.cash < totalCost {
			continue
		}

		// 执行买入
		rc.cash -= totalCost

		rc.trades = append(rc.trades, BacktestTrade{
			RunID:      rc.runID,
			StockCode:  order.Code,
			TradeType:  1, // 买入
			Quantity:   buyQty,
			Price:      buyPrice,
			Amount:     buyAmount,
			Commission: commission,
			TradeDate:  NewDateOnly(date),
			CreatedAt:  time.Now(),
		})

		// 创建持仓 — 调用所有 checker 的 OnEntry 钩子
		pos := &HoldingPosition{
			StockCode:  order.Code,
			Quantity:   buyQty,
			EntryPrice: buyPrice,
			EntryDate:  date,
			HoldDays:   0,
		}
		for _, c := range exitChain {
			if h, ok := c.(EntryHook); ok {
				h.OnEntry(pos, buyPrice, date)
			}
		}
		rc.holdings[order.Code] = pos
	}
}

// calcVolatility 从 StockSource 计算近 20 日波动率
func calcVolatility(src indicator.StockSource) float64 {
	klines, err := src.GetDailyKline()
	if err != nil || len(klines) < 2 {
		return 1.0 // 默认波动率
	}
	// 取最近 20 条
	start := 0
	if len(klines) > 20 {
		start = len(klines) - 20
	}
	klines = klines[start:]
	// 计算日收益率的标准差（Close 是 int 类型，单位：分）
	returns := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		if klines[i-1].Close > 0 {
			prev := float64(klines[i-1].Close)
			curr := float64(klines[i].Close)
			r := (curr - prev) / prev
			returns = append(returns, r)
		}
	}
	if len(returns) == 0 {
		return 1.0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(returns))
	return math.Sqrt(variance) * math.Sqrt(250) // 年化波动率
}

// executePartialSell 执行部分卖出
func (s *Service) executePartialSell(rc *runContext, code string, pos *HoldingPosition, bar DayBar, decision *ExitDecision, date string) {
	sellQty := decision.Quantity
	if sellQty >= pos.Quantity {
		sellQty = pos.Quantity
	}

	sellPrice := decision.Price
	sellAmount := sellPrice * float64(sellQty)

	commission := calcCommission(sellAmount, rc.commissionRate, rc.minCommission)
	stampTax := calcStampTax(sellAmount)

	rc.cash += sellAmount - commission - stampTax

	// 计算盈亏（按持仓均价）
	entryAmount := pos.EntryPrice * float64(sellQty)
	profitLoss := sellAmount - entryAmount - commission - stampTax
	profitLossPct := profitLoss / entryAmount * 100

	exitReason := decision.Reason
	preExitPrice := sellPrice

	rc.trades = append(rc.trades, BacktestTrade{
		RunID:         rc.runID,
		StockCode:     code,
		TradeType:     2, // 卖出
		Quantity:      sellQty,
		Price:         sellPrice,
		Amount:        sellAmount,
		Commission:    commission,
		StampTax:      stampTax,
		TradeDate:     NewDateOnly(date),
		ExitReason:    &exitReason,
		PreExitPrice:  &preExitPrice,
		ProfitLoss:    &profitLoss,
		ProfitLossPct: &profitLossPct,
		CreatedAt:     time.Now(),
	})

	// 减少持仓数量，不删除持仓
	pos.Quantity -= sellQty
}

// ============================================================================
//  收尾与失败处理
// ============================================================================

// finalize 完成回测，计算指标并批量持久化
func (s *Service) finalize(runID uint64, rc *runContext, tradingDays int) {
	// 计算绩效指标
	metrics := CalcMetrics(rc.dailySnapshots, rc.trades, rc.initialCapital, tradingDays)

	// 统计卖出原因
	stopLossCount := 0
	takeProfitCount := 0
	timeExitCount := 0
	for _, t := range rc.trades {
		if t.TradeType == 2 && t.ExitReason != nil {
			switch *t.ExitReason {
			case "stop_loss":
				stopLossCount++
			case "take_profit":
				takeProfitCount++
			case "time_exit":
				timeExitCount++
			}
		}
	}

	// 持久化
	s.dao.CreateTrades(rc.trades)
	// snapshots 已逐日入库，此处不再批量写入

	finalEquity := 0.0
	if len(rc.dailySnapshots) > 0 {
		finalEquity = rc.dailySnapshots[len(rc.dailySnapshots)-1].TotalEquity
	}

	run := &BacktestRun{
		ID:              runID,
		Status:          "done",
		ProgressPct:     100,
		FinalEquity:     &finalEquity,
		TotalReturn:     &metrics.TotalReturn,
		AnnualReturn:    &metrics.AnnualReturn,
		MaxDrawdown:     &metrics.MaxDrawdown,
		SharpeRatio:     &metrics.SharpeRatio,
		WinRate:         &metrics.WinRate,
		ProfitFactor:    &metrics.ProfitFactor,
		TradeCount:      metrics.TradeCount,
		StopLossCount:   stopLossCount,
		TakeProfitCount: takeProfitCount,
		TimeExitCount:   timeExitCount,
		UpdatedAt:       time.Now(),
	}
	s.dao.UpdateRun(run)
}

// failRun 标记回测失败
func (s *Service) failRun(runID uint64, errMsg string) {
	s.dao.UpdateRun(&BacktestRun{
		ID:           runID,
		Status:       "failed",
		ErrorMessage: errMsg,
		UpdatedAt:    time.Now(),
	})
}

// =========================== 辅助函数 ===========================

func ptrFloat(v float64) *float64 { return &v }
