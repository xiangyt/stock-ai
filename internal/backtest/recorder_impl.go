package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/db"
)

// ConsoleTradeRecorder 控制台交易记录器。
type ConsoleTradeRecorder struct{}

// NewConsoleTradeRecorder 创建控制台记录器。
func NewConsoleTradeRecorder() *ConsoleTradeRecorder { return &ConsoleTradeRecorder{} }

// RecordTrade 打印交易到 stdout。
func (r *ConsoleTradeRecorder) RecordTrade(
	_ context.Context, _ uint64, trade Trade,
) error {
	d := "买入"
	if trade.Type == 2 {
		d = "卖出"
	}
	p := ""
	if trade.Profit != 0 {
		p = fmt.Sprintf(" 盈亏:%.2f", trade.Profit)
	}
	fmt.Printf("[交易] %s | %s %s %d股 @%.2f 金额:%.2f%s\n",
		trade.Date, d, trade.Code, trade.Quantity, trade.Price, trade.Amount, p)
	return nil
}

// RecordSnapshot 打印每日权益到 stdout。
func (r *ConsoleTradeRecorder) RecordSnapshot(
	_ context.Context, snap EquitySnapshot,
) error {
	fmt.Printf("[快照] %s | 权益:%.2f 现金:%.2f 市值:%.2f 持仓:%d\n",
		snap.Date, snap.TotalEquity, snap.Cash, snap.MarketValue, snap.PositionCount)
	return nil
}

// RecordMetrics 打印最终指标。
func (r *ConsoleTradeRecorder) RecordMetrics(
	_ context.Context, _ uint64, m RunResult,
) error {
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("回测完成: 总收益=%.2f%% 年化=%.2f%% 回撤=%.2f%% 胜率=%.2f%% 交易=%d\n",
		m.TotalReturn, m.AnnualReturn, m.MaxDrawdown, m.WinRate, m.TradeCount)
	fmt.Println(strings.Repeat("=", 50))
	return nil
}

// UpdateProgress 打印进度到 stdout。
func (r *ConsoleTradeRecorder) UpdateProgress(
	_ context.Context, _ uint64, pct int,
) error {
	if pct%10 == 0 || pct == 100 {
		fmt.Printf("\r进度: %d%%", pct)
		if pct == 100 { fmt.Println() }
	}
	return nil
}

// DBTradeRecorder 数据库交易记录器。
type DBTradeRecorder struct{ runID uint64 }

// NewDBTradeRecorder 创建 DB 记录器。
func NewDBTradeRecorder(runID uint64) *DBTradeRecorder {
	return &DBTradeRecorder{runID: runID}
}

// RunID 返回当前 runID。
func (r *DBTradeRecorder) RunID() uint64 { return r.runID }

// CreateRun 创建 backtest_runs 记录。
func (r *DBTradeRecorder) CreateRun(
	ctx context.Context, req RunRequest,
) (uint64, error) {
	now := time.Now()
	run := struct {
		StartDate, EndDate string
		InitialCapital      float64
		Status              string
		CreatedAt, UpdatedAt time.Time
	}{req.StartDate, req.EndDate, req.InitialCapital, "pending", now, now}
	if err := db.GetDB().WithContext(ctx).Table("backtest_runs").Create(&run).Error; err != nil {
		return 0, err
	}
	return 0, nil
}

// FailRun 标记回测失败。
func (r *DBTradeRecorder) FailRun(ctx context.Context, runID uint64, msg string) {
	updates := map[string]any{
		"status": "failed", "error_message": msg, "updated_at": time.Now(),
	}
	db.GetDB().WithContext(ctx).Table("backtest_runs").
		Where("id = ?", runID).Updates(updates)
}

// RecordTrade 写入 backtest_trades 表。
func (r *DBTradeRecorder) RecordTrade(
	ctx context.Context, _ uint64, trade Trade,
) error {
	sql := `INSERT INTO backtest_trades (run_id,stock_code,trade_type,quantity,price,
		amount,commission,stamp_tax,trade_date,exit_reason,profit_loss,profit_loss_pct,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,NOW())`
	return db.GetDB().WithContext(ctx).Exec(sql,
		r.runID, trade.Code, trade.Type, trade.Quantity, trade.Price,
		trade.Amount, trade.Commission, trade.StampTax, trade.Date,
		trade.Reason, trade.Profit, trade.ProfitPct).Error
}

// RecordSnapshot 写入 daily_snapshots 表。
func (r *DBTradeRecorder) RecordSnapshot(
	ctx context.Context, snap EquitySnapshot,
) error {
	sql := `INSERT INTO daily_snapshots (run_id,snap_date,total_equity,cash,
		market_value,position_count,daily_return,cumulative_return,created_at)
		VALUES (?,?,?,?,?,?,?,?,NOW())`
	return db.GetDB().WithContext(ctx).Exec(sql,
		r.runID, snap.Date, snap.TotalEquity, snap.Cash,
		snap.MarketValue, snap.PositionCount, snap.DailyReturn, snap.CumulativeReturn).Error
}

// RecordMetrics 更新 backtest_runs 最终指标。
func (r *DBTradeRecorder) RecordMetrics(
	ctx context.Context, _ uint64, m RunResult,
) error {
	sql := `UPDATE backtest_runs SET status='done',progress_pct=100,
		total_return=?,annual_return=?,max_drawdown=?,win_rate=?,
		profit_factor=?,trade_count=?,updated_at=NOW() WHERE id=?`
	return db.GetDB().WithContext(ctx).Exec(sql,
		m.TotalReturn, m.AnnualReturn, m.MaxDrawdown,
		m.WinRate, m.ProfitFactor, m.TradeCount, r.runID).Error
}

// UpdateProgress 更新进度。
func (r *DBTradeRecorder) UpdateProgress(
	ctx context.Context, _ uint64, pct int,
) error {
	return db.GetDB().WithContext(ctx).Exec(
		"UPDATE backtest_runs SET progress_pct=?,updated_at=NOW() WHERE id=?",
		pct, r.runID).Error
}
