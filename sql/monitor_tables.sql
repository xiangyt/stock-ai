-- monitor_configs: 盯盘助手配置主表
CREATE TABLE IF NOT EXISTS `monitor_configs` (
    `id`         INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `uid`        INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID',
    `name`       VARCHAR(100) NOT NULL COMMENT '监控名称',
    `scope`      VARCHAR(20)  NOT NULL DEFAULT 'held' COMMENT '监控范围: held=持仓股 custom=自选股票',
    `stocks`     TEXT         COMMENT '自选股票代码(JSON数组): scope=custom时必填',
    `rules`      TEXT         NOT NULL COMMENT '告警规则配置(JSON)',
    `cooldown`   TEXT         NOT NULL COMMENT '冷却策略配置(JSON): {interval_minutes, daily_max}',
    `template`   TEXT         COMMENT '自定义推送模板(空则用默认)',
    `is_active`  TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '是否启用: 0=停用 1=启用',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME     NULL COMMENT '软删除时间',
    PRIMARY KEY (`id`),
    INDEX `idx_uid` (`uid`),
    INDEX `idx_is_active` (`is_active`),
    INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='盯盘助手配置表';

-- monitor_config_bots: 监控配置与推送机器人的关联表
CREATE TABLE IF NOT EXISTS `monitor_config_bots` (
    `id`         INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `config_id`  INT UNSIGNED NOT NULL COMMENT '监控配置ID',
    `bot_id`     INT UNSIGNED NOT NULL COMMENT '推送机器人ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE INDEX `idx_config_bot` (`config_id`, `bot_id`),
    INDEX `idx_config_id` (`config_id`),
    INDEX `idx_bot_id` (`bot_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='监控配置关联机器人表';
