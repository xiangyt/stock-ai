-- =============================================
-- 股票每日快照表: 估值/股本/市值等衍生指标
-- 数据来源: 从 K线 + 财报 + 股本变动 计算得出
-- 日期字段统一使用 INT(8) YYYYMMDD 格式
-- =============================================

CREATE TABLE IF NOT EXISTS stock_daily_snapshot (
    stock_code          CHAR(10)        NOT NULL COMMENT '股票代码',
    trade_date          INT(8)          NOT NULL DEFAULT 0 COMMENT '交易日期 YYYYMMDD',

    -- 估值指标 (倍)
    pe_dynamic          DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '市盈率(动态)',
    pe_static           DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '市盈率(静态)',
    pe_ttm              DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '市盈率(TTM)',
    ps_ttm              DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '市销率(TTM)',
    pb                  DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '市净率',

    -- 盈利能力指标 (%)
    roe                 DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '净资产收益率(%)',
    roa                 DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '总资产收益率(%)',
    gross_margin        DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '毛利率(%)',
    net_margin          DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '净利率(%)',

    -- 每股指标 (元)
    bvps                DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '每股净资产',
    basic_eps           DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '基本每股收益',

    -- 财报当期数据 (元)
    parent_net_profit   DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '归母净利润',
    deduct_net_profit   DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '扣非净利润',
    total_revenue       DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '营业总收入',

    -- 股本数据 (股)
    total_shares        BIGINT          NOT NULL DEFAULT 0 COMMENT '总股本',
    float_shares        BIGINT          NOT NULL DEFAULT 0 COMMENT '流通A股',

    -- 市值数据 (元)
    total_market_cap    DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '总市值',
    circulate_market_cap DECIMAL(30,4)  NOT NULL DEFAULT 0 COMMENT '流通市值',

    -- 偿债能力指标 (%)
    debt_ratio          DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '资产负债率(%)',

    PRIMARY KEY (stock_code, trade_date),
    INDEX idx_trade_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='股票每日估值快照';
