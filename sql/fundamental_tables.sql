-- =============================================
-- 基本面数据表结构：财报/股东户数/股本变动
-- 日期字段统一使用 INT(8) YYYYMMDD 格式
-- =============================================

-- ---------- 财报(业绩报表) ----------
CREATE TABLE IF NOT EXISTS performance_reports (
    stock_code            CHAR(10)        NOT NULL COMMENT '股票代码',
    report_date           INT(8)          NOT NULL DEFAULT 0 COMMENT '报告期 YYYYMMDD',
    report_type           VARCHAR(20)     NOT NULL DEFAULT '' COMMENT '报告类型(年报/一季报/中报/三季报)',
    report_name           VARCHAR(50)     NOT NULL DEFAULT '' COMMENT '报告期名称(2025年报)',
    currency              CHAR(10)        NOT NULL DEFAULT 'CNY' COMMENT '货币单位',

    -- 每股指标
    basic_eps             DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '基本每股收益(元)',
    deducted_eps          DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '扣非每股收益(元)',
    diluted_eps           DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '摊薄每股收益(元)',
    bvps                  DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '每股净资产(元)',
    equity_reserve        DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '每股公积金',
    undistributed_profit  DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '每股未分配利润(元)',
    ocfps                 DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '每股经营现金流(元)',

    -- 成长能力
    total_revenue         DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '营业总收入(元)',
    gross_profit          DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '毛利润(元)',
    parent_net_profit     DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '归属净利润(元)',
    deduct_net_profit     DECIMAL(30,4)   NOT NULL DEFAULT 0 COMMENT '扣非净利润(元)',
    revenue_yoy           DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '营收同比(%)',
    parent_net_profit_yoy DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '归母净利同比(%)',
    deduct_net_profit_yoy DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '扣非净利同比(%)',

    -- 盈利能力
    roe_w                 DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '净资产收益率-加权(%)',
    roe_dw                DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '净资产收益率-扣非加权(%)',
    roa                   DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '总资产收益率(%)',
    gross_margin          DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '销售毛利率(%)',
    net_margin            DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '销售净利率(%)',

    -- 偿债能力
    current_ratio         DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '流动比率(倍)',
    quick_ratio           DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '速动比率(倍)',
    debt_ratio            DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '资产负债率(%)',

    -- 现金流
    ocf_to_revenue        DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '经营净现金流/营业收入',

    PRIMARY KEY (stock_code, report_date),
    INDEX idx_report_date (report_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='财报(业绩报表)数据';

-- ---------- 股东户数 ----------
CREATE TABLE IF NOT EXISTS shareholder_counts (
    stock_code                   CHAR(10)        NOT NULL COMMENT '股票代码',
    end_date                     INT(8)          NOT NULL DEFAULT 0 COMMENT '统计截止日 YYYYMMDD',
    security_name                VARCHAR(50)     NOT NULL DEFAULT '' COMMENT '证券简称',

    holder_num                   BIGINT          NOT NULL DEFAULT 0 COMMENT '股东人数(户)',
    holder_num_change_pct        DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '较上期变化(%)',
    avg_free_shares              BIGINT          NOT NULL DEFAULT 0 COMMENT '人均流通股(股)',
    avg_free_shares_change_pct   DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '较上期变化(%)',
    hold_focus                   VARCHAR(20)     NOT NULL DEFAULT '' COMMENT '筹码集中度',
    price                        DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '股价(元)(报告期末)',
    avg_hold_amount              DECIMAL(20,4)   NOT NULL DEFAULT 0 COMMENT '人均持股市值(元)',
    hold_ratio_total             DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '十大股东持股合计(%)',
    free_hold_ratio_total        DECIMAL(10,4)   NOT NULL DEFAULT 0 COMMENT '十大流通股东持股合计(%)',

    PRIMARY KEY (stock_code, end_date),
    INDEX idx_end_date (end_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='股东户数数据';

-- ---------- 股本变动 ----------
CREATE TABLE IF NOT EXISTS share_changes (
    stock_code           CHAR(10)        NOT NULL COMMENT '股票代码',
    change_date          INT(8)          NOT NULL DEFAULT 0 COMMENT '变动日期 YYYYMMDD',
    change_reason        VARCHAR(200)    NOT NULL DEFAULT '' COMMENT '变动原因',
    total_shares         BIGINT          NOT NULL DEFAULT 0 COMMENT '总股本(股)',
    limited_shares       BIGINT          NOT NULL DEFAULT 0 COMMENT '流通受限股份(股)',
    unlimited_shares     BIGINT          NOT NULL DEFAULT 0 COMMENT '已流通股份(股)',
    float_a_shares       BIGINT          NOT NULL DEFAULT 0 COMMENT '已上市流通A股(股)',

    PRIMARY KEY (stock_code, change_date),
    INDEX idx_change_date (change_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='股本变动数据';