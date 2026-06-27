package backtest

import (
	"context"
	"fmt"

	"stock-ai/utils"
)

// ============================================================================
//  Run — 同步执行完整回测
// ============================================================================

func (e *defaultEngine) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	tradingDays, err := utils.GetTradingDays(req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("get trading days: %w", err)
	}
	if len(tradingDays) == 0 {
		return nil, fmt.Errorf("no trading days in range")
	}
	req.Workload = len(tradingDays) * len(req.StockPool)

	if e.positionManager == nil {
		e.positionManager = NewEqualPositionManager(req.InitialCapital, 5)
	}

	rc := &runContext{
		initialCapital: req.InitialCapital,
		trades:         make([]Trade, 0),
	}

	total := len(tradingDays)
	prevEquity := req.InitialCapital
	for i, date := range tradingDays {
		prevEquity = e.processDay(ctx, rc, date, req, prevEquity)
		pct := (i + 1) * 100 / total
		e.updateProgress(ctx, 0, pct)
	}
	// 最后一天强制清仓
	e.forceClose(rc, tradingDays[len(tradingDays)-1], req.StockPool)

	result := e.calcMetrics(rc, req.InitialCapital, len(tradingDays))
	result.Trades = rc.trades
	result.Snapshots = rc.dailySnapshots
	return result, nil
}

// ============================================================================
//  日级处理
// ============================================================================

// processDay 处理单个交易日，返回当日权益。
func (e *defaultEngine) processDay(
	ctx context.Context, rc *runContext, date string,
	req RunRequest, prevEquity float64,
) float64 {
	acct := e.positionManager.Account()

	snapshots, bars := e.loadDay(ctx, date, req.StockPool, acct.Holdings)
	e.checkExits(rc, bars, date)

	if len(snapshots) > 0 && len(req.EntryConfigs) > 0 {
		e.checkEntries(ctx, rc, snapshots, bars, req.EntryConfigs, date)
	}

	prices := barPrices(bars)
	marketValue := acct.MarketValue(prices)
	totalEquity := acct.Cash + marketValue

	e.recordDay(rc, acct, date, totalEquity, marketValue, prevEquity, req.InitialCapital)
	return totalEquity
}

// recordDay 记录当日快照。
func (e *defaultEngine) recordDay(
	rc *runContext, acct *Account, date string,
	totalEquity, marketValue, prevEquity, initialCapital float64,
) {
	dayReturn := 0.0
	if prevEquity > 0 {
		dayReturn = (totalEquity - prevEquity) / prevEquity * 100
	}
	snap := EquitySnapshot{
		Date:             date,
		TotalEquity:      totalEquity,
		Cash:             acct.Cash,
		MarketValue:      marketValue,
		PositionCount:    acct.HoldingCount(),
		DailyReturn:      dayReturn,
		CumulativeReturn: (totalEquity - initialCapital) / initialCapital * 100,
	}
	rc.dailySnapshots = append(rc.dailySnapshots, snap)
	if e.tradeRecorder != nil {
		e.tradeRecorder.RecordSnapshot(context.TODO(), snap)
	}
}

// updateProgress 向 TradeRecorder 上报回测进度。
func (e *defaultEngine) updateProgress(ctx context.Context, runID uint64, pct int) {
	if e.tradeRecorder != nil {
		e.tradeRecorder.UpdateProgress(ctx, runID, pct)
	}
}

// ============================================================================
//  卖出检查
// ============================================================================

func (e *defaultEngine) checkExits(rc *runContext, bars map[string]bar, date string) {
	if e.exitExecutor == nil {
		return
	}
	acct := e.positionManager.Account()
	for code, pos := range acct.Holdings {
		b, ok := bars[code]
		if !ok {
			continue
		}
		bar := DayBar{
			Date: date, Open: b.open, High: b.high, Low: b.low, Close: b.close,
		}

		// 每日更新钩子（移动止损最高价等）
		if h, ok := e.exitExecutor.(ExitDailyHook); ok {
			h.OnDailyUpdate(pos, bar)
		}

		decision := e.exitExecutor.Check(context.TODO(), pos, bar)
		if decision == nil {
			continue
		}
		e.executeSell(rc, code, pos, decision, date)
	}
}

// ============================================================================
//  买入选股
// ============================================================================

func (e *defaultEngine) checkEntries(
	ctx context.Context, rc *runContext,
	snapshots []*StockSnapshot, bars map[string]bar,
	configs []SignalConfig, date string,
) {
	acct := e.positionManager.Account()

	candidates := make([]*StockSnapshot, 0)
	candidateBars := make(map[string]bar)
	for _, snap := range snapshots {
		if _, held := acct.Holdings[snap.Code]; !held {
			candidates = append(candidates, snap)
			if b, ok := bars[snap.Code]; ok {
				candidateBars[snap.Code] = b
			}
		}
	}
	if len(candidates) == 0 {
		return
	}

	summary, err := e.screener.Screen(ctx, candidates, configs)
	if err != nil {
		return
	}
	cInfos := make([]CandidateInfo, 0)
	for _, r := range summary.Results {
		if r.Passed {
			cInfos = append(cInfos, CandidateInfo{
				Code:  r.Code,
				Price: candidateBars[r.Code].close,
			})
		}
	}
	if len(cInfos) == 0 {
		return
	}

	orders, err := e.positionManager.Allocate(ctx, cInfos)
	if err != nil {
		return
	}

	for _, order := range orders {
		b, ok := candidateBars[order.Code]
		if !ok {
			continue
		}
		e.executeBuy(rc, &b, order, date)
	}
}

// ============================================================================
//  交易执行
// ============================================================================

func (e *defaultEngine) executeSell(
	rc *runContext, code string, pos *HoldingPosition,
	decision *ExitDecision, date string,
) {
	acct := e.positionManager.Account()

	qty := decision.Quantity
	if qty <= 0 || qty > pos.Quantity {
		qty = pos.Quantity
	}
	price := decision.Price
	if price <= 0 {
		price = pos.EntryPrice
	}
	amount := float64(qty) * price
	fee := e.calcFee(amount, true)

	entryAmt := float64(qty) * pos.EntryPrice
	profit := amount - entryAmt - fee
	profitPct := 0.0
	if entryAmt > 0 {
		profitPct = profit / entryAmt * 100
	}

	acct.AddCash(amount - fee)

	rc.trades = append(rc.trades, Trade{
		Code: code, Type: 2, Quantity: qty, Price: price, Amount: amount,
		Commission: fee, StampTax: amount * 0.001, Date: date,
		Profit: profit, ProfitPct: profitPct, Reason: decision.Reason,
	})

	if qty == pos.Quantity {
		acct.RemoveHolding(code)
	} else {
		acct.UpdateHoldingQty(code, pos.Quantity-qty)
	}
}

func (e *defaultEngine) executeBuy(
	rc *runContext, b *bar, order Order, date string,
) {
	if order.Quantity <= 0 || b.close <= 0 {
		return
	}
	amount := float64(order.Quantity) * b.close
	fee := e.calcFee(amount, false)

	acct := e.positionManager.Account()
	if acct.Cash < amount+fee {
		return
	}
	acct.DeductCash(amount + fee)

	rc.trades = append(rc.trades, Trade{
		Code: order.Code, Type: 1, Quantity: order.Quantity,
		Price: b.close, Amount: amount, Commission: fee, Date: date,
	})
	pos := &HoldingPosition{
		Code: order.Code, Quantity: order.Quantity,
		EntryPrice: b.close, EntryDate: date,
	}
	acct.AddHolding(pos)

	// 入场钩子（止损价初始化等）
	if e.exitExecutor != nil {
		if h, ok := e.exitExecutor.(ExitEntryHook); ok {
			h.OnEntry(pos, b.close, date)
		}
	}
}

func (e *defaultEngine) calcFee(amount float64, isSell bool) float64 {
	if isSell {
		return e.feeCalculator.SellFee(amount)
	}
	return e.feeCalculator.BuyFee(amount)
}

// ============================================================================
//  强制清仓 & 异步运行
// ============================================================================

// forceClose 回测最后一天按收盘价强制清仓所有持仓。
func (e *defaultEngine) forceClose(rc *runContext, lastDate string, pool []string) {
	acct := e.positionManager.Account()
	if acct.HoldingCount() == 0 {
		return
	}

	snaps, bars := e.loadDay(context.TODO(), lastDate, pool, acct.Holdings)
	for code, pos := range acct.Holdings {
		b, ok := bars[code]
		if !ok {
			continue
		}
		amount := float64(pos.Quantity) * b.close
		fee := e.calcFee(amount, true)
		entryAmt := float64(pos.Quantity) * pos.EntryPrice
		profit := amount - entryAmt - fee
		profitPct := 0.0
		if entryAmt > 0 {
			profitPct = profit / entryAmt * 100
		}

		acct.AddCash(amount - fee)
		rc.trades = append(rc.trades, Trade{
			Code: code, Type: 2, Quantity: pos.Quantity,
			Price: b.close, Amount: amount, Commission: fee,
			StampTax: amount * 0.001, Date: lastDate,
			Profit: profit, ProfitPct: profitPct,
			Reason: "force_close",
		})
		acct.RemoveHolding(code)
	}
	_ = snaps
}

// runAsync 异步执行回测（goroutine 内），含 panic 恢复。
func (e *defaultEngine) runAsync(ctx context.Context, runID uint64, req RunRequest) {
	defer func() {
		if r := recover(); r != nil {
			if rec, ok := e.tradeRecorder.(*DBTradeRecorder); ok {
				rec.FailRun(ctx, runID, fmt.Sprintf("panic: %v", r))
			}
		}
	}()

	result, err := e.Run(ctx, req)
	if err != nil {
		if rec, ok := e.tradeRecorder.(*DBTradeRecorder); ok {
			rec.FailRun(ctx, runID, err.Error())
		}
		return
	}

	// 持久化最终指标到 DB
	_ = result
	if e.tradeRecorder != nil {
		e.tradeRecorder.RecordMetrics(ctx, runID, *result)
	}
}
