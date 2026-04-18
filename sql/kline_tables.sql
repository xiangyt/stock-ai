-- =============================================
-- K线多周期表结构：日 / 周 / 月 / 年
-- 价格单位: 分(INT), 成交量: 股(BIGINT), 成交额: 分(BIGINT)
-- =============================================

-- ---------- 日K线 ----------
CREATE TABLE IF NOT EXISTS daily_kline (
    stock_code    CHAR(10)      NOT NULL COMMENT '股票代码',
    trade_date    INT(8)        NOT NULL DEFAULT 0 COMMENT '交易日期 YYYYMMDD',
    open          INT           NOT NULL DEFAULT 0 COMMENT '开盘价(分)',
    high          INT           NOT NULL DEFAULT 0 COMMENT '最高价(分)',
    low           INT           NOT NULL DEFAULT 0 COMMENT '最低价(分)',
    close         INT           NOT NULL DEFAULT 0 COMMENT '收盘价(分)',
    volume        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交量(股)',
    amount        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交额(分)',
    turnover_rate DECIMAL(8,4)  NOT NULL DEFAULT 0 COMMENT '换手率',

    PRIMARY KEY (stock_code, trade_date),
    INDEX idx_trade_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='日K线数据';

-- ---------- 周K线 ----------
CREATE TABLE IF NOT EXISTS weekly_kline (
    stock_code    CHAR(10)      NOT NULL COMMENT '股票代码',
    trade_date    INT(8)        NOT NULL DEFAULT 0 COMMENT '周最后交易日 YYYYMMDD',
    open          INT           NOT NULL DEFAULT 0 COMMENT '开盘价(分)',
    high          INT           NOT NULL DEFAULT 0 COMMENT '最高价(分)',
    low           INT           NOT NULL DEFAULT 0 COMMENT '最低价(分)',
    close         INT           NOT NULL DEFAULT 0 COMMENT '收盘价(分)',
    volume        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交量(股)',
    amount        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交额(分)',
    turnover_rate DECIMAL(8,4)  NOT NULL DEFAULT 0 COMMENT '换手率',

    PRIMARY KEY (stock_code, trade_date),
    INDEX idx_trade_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='周K线数据';

-- ---------- 月K线 ----------
CREATE TABLE IF NOT EXISTS monthly_kline (
    stock_code    CHAR(10)      NOT NULL COMMENT '股票代码',
    trade_date    INT(8)        NOT NULL DEFAULT 0 COMMENT '月最后交易日 YYYYMMDD',
    open          INT           NOT NULL DEFAULT 0 COMMENT '开盘价(分)',
    high          INT           NOT NULL DEFAULT 0 COMMENT '最高价(分)',
    low           INT           NOT NULL DEFAULT 0 COMMENT '最低价(分)',
    close         INT           NOT NULL DEFAULT 0 COMMENT '收盘价(分)',
    volume        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交量(股)',
    amount        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交额(分)',
    turnover_rate DECIMAL(8,4)  NOT NULL DEFAULT 0 COMMENT '换手率',

    PRIMARY KEY (stock_code, trade_date),
    INDEX idx_trade_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='月K线数据';

-- ---------- 年K线 ----------
CREATE TABLE IF NOT EXISTS yearly_kline (
    stock_code    CHAR(10)      NOT NULL COMMENT '股票代码',
    trade_date    INT(8)        NOT NULL DEFAULT 0 COMMENT '年最后一个交易日 YYYYMMDD',
    open          INT           NOT NULL DEFAULT 0 COMMENT '开盘价(分)',
    high          INT           NOT NULL DEFAULT 0 COMMENT '最高价(分)',
    low           INT           NOT NULL DEFAULT 0 COMMENT '最低价(分)',
    close         INT           NOT NULL DEFAULT 0 COMMENT '收盘价(分)',
    volume        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交量(股)',
    amount        BIGINT        NOT NULL DEFAULT 0 COMMENT '成交额(分)',
    turnover_rate DECIMAL(8,4)  NOT NULL DEFAULT 0 COMMENT '换手率',

    PRIMARY KEY (stock_code, trade_date),
    INDEX idx_trade_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='年K线数据';
