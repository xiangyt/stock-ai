-- ============================================================================
--  数据采集模块 DDL
--  包含: data_collect_tasks(采集任务配置), data_collect_bots(关联机器人)
-- ============================================================================

-- 1. 数据采集任务配置表
CREATE TABLE IF NOT EXISTS data_collect_tasks (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    name       VARCHAR(100)    NOT NULL                COMMENT '任务名称',
    cron_expr  VARCHAR(100)    NOT NULL                COMMENT '6段秒级 cron 表达式',
    params     TEXT                                    COMMENT 'JSON 格式执行参数',
    is_active  TINYINT(1)      NOT NULL DEFAULT 0      COMMENT '是否启用: 0=停用 1=启用',
    created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='数据采集任务配置表';

-- 2. 数据采集任务-机器人关联表 (M2M)
CREATE TABLE IF NOT EXISTS data_collect_bots (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    task_id    BIGINT UNSIGNED NOT NULL                COMMENT '采集任务ID',
    bot_id     BIGINT UNSIGNED NOT NULL                COMMENT '推送机器人ID',
    created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE INDEX idx_dc_task_bot (task_id, bot_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='数据采集任务关联机器人表';
