-- ============================================================================
--  策略回测系统 — 建表 DDL
--  日期: 2026-06-15
--  说明: strategies 表的 exit_rules / position_rules 字段已在 strategy_tables.sql 中定义
-- ============================================================================

-- 1. 回测运行记录表
CREATE TABLE IF NOT EXISTS backtest_runs (
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    strategy_id      BIGINT NOT NULL,
    uid              BIGINT DEFAULT 0,
    stock_pool       JSON NOT NULL,
    start_date       DATE NOT NULL,
    end_date         DATE NOT NULL,
    initial_capital  DECIMAL(20,4) NOT NULL,
    final_equity     DECIMAL(20,4),
    exit_rules       JSON,
    position_rules   JSON,
    status           VARCHAR(20) DEFAULT 'pending',
    progress_pct     INT DEFAULT 0,
    error_message    TEXT,
    total_return     DECIMAL(10,4),
    annual_return    DECIMAL(10,4),
    max_drawdown     DECIMAL(10,4),
    sharpe_ratio     DECIMAL(10,4),
    win_rate         DECIMAL(10,4),
    profit_factor    DECIMAL(10,4),
    trade_count      INT DEFAULT 0,
    stop_loss_count  INT DEFAULT 0,
    take_profit_count INT DEFAULT 0,
    time_exit_count  INT DEFAULT 0,
    created_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL,
    INDEX idx_strategy (strategy_id),
    INDEX idx_uid_status (uid, status)
);

-- 2. 回测交易明细表
CREATE TABLE IF NOT EXISTS backtest_trades (
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id           BIGINT NOT NULL,
    stock_code       CHAR(6) NOT NULL,
    trade_type       TINYINT NOT NULL,
    quantity         INT NOT NULL,
    price            DECIMAL(14,4) NOT NULL,
    amount           DECIMAL(20,4) NOT NULL,
    commission       DECIMAL(14,4) DEFAULT 0,
    stamp_tax        DECIMAL(14,4) DEFAULT 0,
    trade_date       DATE NOT NULL,
    exit_reason      VARCHAR(20),
    pre_exit_price   DECIMAL(14,4),
    profit_loss      DECIMAL(14,4),
    profit_loss_pct  DECIMAL(10,4),
    created_at       DATETIME NOT NULL,
    INDEX idx_run (run_id),
    INDEX idx_run_stock (run_id, stock_code),
    INDEX idx_run_date (run_id, trade_date)
);

-- 3. 每日净值快照表
CREATE TABLE IF NOT EXISTS daily_snapshots (
    id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id              BIGINT NOT NULL,
    snap_date           DATE NOT NULL,
    total_equity        DECIMAL(20,4) NOT NULL,
    cash                DECIMAL(20,4) NOT NULL,
    market_value        DECIMAL(20,4) NOT NULL,
    position_count      INT DEFAULT 0,
    daily_return        DECIMAL(10,4),
    cumulative_return   DECIMAL(10,4),
    benchmark_value     DECIMAL(20,4),
    created_at          DATETIME NOT NULL,
    INDEX idx_run_date (run_id, snap_date)
);
