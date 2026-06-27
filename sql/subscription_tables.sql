-- ============================================================================
--  策略订阅模块 DDL
--  包含: strategy_subscriptions(订阅配置), subscription_bots(关联机器人), subscription_logs(执行日志)
-- ============================================================================

-- 1. 策略订阅配置表
CREATE TABLE IF NOT EXISTS strategy_subscriptions (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    uid                BIGINT UNSIGNED NOT NULL                COMMENT '用户ID',
    name               VARCHAR(100)    NOT NULL                COMMENT '订阅名称',
    strategy_id        BIGINT UNSIGNED NOT NULL                COMMENT '关联策略ID',
    scope              VARCHAR(20)     NOT NULL DEFAULT 'all'  COMMENT '监控范围: all=全部A股 held=持仓股 custom=自选股票',
    custom_stocks      TEXT                                    COMMENT '自选股票代码 JSON 数组(scope=custom时必填)',
    preset_type        VARCHAR(20)     NOT NULL DEFAULT 'every_30min' COMMENT '预设频率: every_15min/every_30min/every_hour/daily_open/daily_close/daily_twice/noon/close_alert/custom',
    cron_expr          VARCHAR(100)                            COMMENT '自定义cron表达式(preset_type=custom时使用)',
    trading_hours_only TINYINT(1)      NOT NULL DEFAULT 1      COMMENT '仅交易时段执行: 0=否 1=是',
    is_active          TINYINT(1)      NOT NULL DEFAULT 1      COMMENT '是否启用: 0=停用 1=启用',
    template           TEXT                                    COMMENT '自定义推送模板(NULL则用默认)',
    last_run_at        DATETIME(3)                             COMMENT '最后运行时间',
    created_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at         DATETIME(3)                             COMMENT '软删除时间',
    PRIMARY KEY (id),
    INDEX idx_strategy_subscriptions_strategy_id (strategy_id),
    INDEX idx_strategy_subscriptions_uid (uid),
    INDEX idx_strategy_subscriptions_is_active (is_active),
    INDEX idx_strategy_subscriptions_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='策略订阅配置表';

-- 2. 订阅-机器人关联表 (M2M)
CREATE TABLE IF NOT EXISTS subscription_bots (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    subscription_id BIGINT UNSIGNED NOT NULL                COMMENT '订阅ID',
    bot_id          BIGINT UNSIGNED NOT NULL                COMMENT '推送机器人ID',
    created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE INDEX idx_sub_bot (subscription_id, bot_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='订阅关联机器人表';

-- 3. 订阅执行日志表
CREATE TABLE IF NOT EXISTS subscription_logs (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    subscription_id BIGINT UNSIGNED NOT NULL                COMMENT '订阅ID',
    run_time        DATETIME(3)     NOT NULL                COMMENT '执行时间',
    finished_at     DATETIME(3)                             COMMENT '完成时间',
    scope           VARCHAR(20)                             COMMENT '执行范围',
    total_scanned   BIGINT          NOT NULL DEFAULT 0      COMMENT '扫描股票总数',
    match_count     BIGINT          NOT NULL DEFAULT 0      COMMENT '匹配股票数',
    match_stocks    TEXT                                    COMMENT '匹配股票列表 JSON',
    duration_ms     BIGINT          NOT NULL DEFAULT 0      COMMENT '执行耗时(毫秒)',
    status          VARCHAR(20)     NOT NULL                COMMENT '状态: success/partial/failed',
    error_msg       TEXT                                    COMMENT '错误信息',
    push_status     TEXT                                    COMMENT '推送状态 JSON',
    created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    INDEX idx_subscription_logs_subscription_id (subscription_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='订阅执行日志表';
