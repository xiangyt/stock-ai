-- ============================
-- stocks 股票基础信息表
-- 与 internal/model/stock.go 保持一致
-- ============================

DROP TABLE IF EXISTS stocks;
CREATE TABLE stocks (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    code                 VARCHAR(20)   NOT NULL DEFAULT '' COMMENT '股票代码',
    name                 VARCHAR(50)   NOT NULL DEFAULT '' COMMENT '股票简称',
    full_name            VARCHAR(100)  NOT NULL DEFAULT '' COMMENT '股票全称',
    english_name         VARCHAR(200)  NOT NULL DEFAULT '' COMMENT '英文名称',
    exchange             VARCHAR(10)   NOT NULL DEFAULT '' COMMENT '交易所(SSE/SZSE/BSE)',
    exchange_name        VARCHAR(50)   NOT NULL DEFAULT '' COMMENT '交易所中文名',
    listing_board        VARCHAR(20)   NOT NULL DEFAULT '' COMMENT '上市板块(main/chinext/star/bse)',
    board_name           VARCHAR(50)   NOT NULL DEFAULT '' COMMENT '板块名称(主板/创业板/科创板/北交所)',
    list_date            VARCHAR(10)   NOT NULL DEFAULT '' COMMENT '上市日期 YYYY-MM-DD',
    delist_date          VARCHAR(10)   NOT NULL DEFAULT '' COMMENT '退市日期 YYYY-MM-DD',
    issue_price          DECIMAL(20,4) NOT NULL DEFAULT 0 COMMENT '发行价(元)',
    issue_pe             DECIMAL(10,4) NOT NULL DEFAULT 0 COMMENT '发行市盈率(倍)',
    issue_pb             DECIMAL(10,4) NOT NULL DEFAULT 0 COMMENT '发行市净率(倍)',
    issue_shares        BIGINT         NOT NULL DEFAULT 0 COMMENT '发行股数(股)',
    industry             VARCHAR(100)  NOT NULL DEFAULT '' COMMENT '所属行业(申万一级)',
    industry_code        VARCHAR(20)   NOT NULL DEFAULT '' COMMENT '行业代码',
    sector               VARCHAR(100)  NOT NULL DEFAULT '' COMMENT '细分行业',
    created              DATETIME(3)   DEFAULT NULL,
    updated              DATETIME(3)   DEFAULT NULL,
    deleted_at           DATETIME(3)   DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_stocks_code        (code),
    KEY        idx_stocks_exchange    (exchange),
    KEY        idx_stocks_listing_board (listing_board),
    KEY        idx_stocks_list_date   (list_date),
    KEY        idx_stocks_industry    (industry),
    KEY        idx_stocks_deleted_at  (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='股票基础信息表';
