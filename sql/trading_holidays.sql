-- ============================================================================
--  A股法定节假日表 DDL
--  存储历年法定节假日日期（包含落在周末的节假日），
--  由 IsTradingDay() 在查询时结合周末判断来判定是否为交易日
--  数据来源: bastengao/chinese-holidays-data (2016-2026) + chinese-calendar (2004-2015) + 手动收集 (2000-2003)
-- ============================================================================

CREATE TABLE IF NOT EXISTS `trading_holidays` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `holiday_date` VARCHAR(10) NOT NULL COMMENT '节假日日期 YYYY-MM-DD',
    `holiday_name` VARCHAR(50) NOT NULL COMMENT '节假日名称',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_holiday_date` (`holiday_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='A股法定节假日表';
