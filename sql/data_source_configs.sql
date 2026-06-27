-- ============================================================================
--  数据源配置表 DDL
--  支持多个数据源，每个数据源可独立配置（API Key、频率限制、配额等）
-- ============================================================================

CREATE TABLE IF NOT EXISTS data_source_configs (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    name          VARCHAR(50)     NOT NULL                COMMENT '数据源标识名: tushare/eastmoney/akshare',
    display_name  VARCHAR(100)    NOT NULL DEFAULT ''     COMMENT '显示名称: Tushare Pro/东方财富/AKShare',
    type          VARCHAR(20)     NOT NULL                COMMENT '类型: api/sdk/web_crawl',
    status        VARCHAR(20)     NOT NULL DEFAULT 'active' COMMENT '状态: active/disabled/error',
    priority      INT             NOT NULL DEFAULT 0      COMMENT '优先级(数字越小优先级越高)',
    config        TEXT                                    COMMENT 'JSON格式配置(API Key、URL等)',
    rate_limit    INT             NOT NULL DEFAULT 60     COMMENT '每分钟请求限制',
    daily_quota   INT             NOT NULL DEFAULT 0      COMMENT '每日调用配额(0=无限制)',
    used_quota    INT             NOT NULL DEFAULT 0      COMMENT '已使用配额',
    quota_reset_at DATETIME(3)                            COMMENT '配额重置时间',
    description   VARCHAR(500)    NOT NULL DEFAULT ''     COMMENT '描述',
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE INDEX idx_data_source_configs_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='数据源配置表';
