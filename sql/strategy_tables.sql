-- =============================================
-- 选股策略表: 存储用户创建的选股条件组合
-- conditions 字段存储 JSON 格式的信号数组
-- 支持软删除（deleted_at）
-- =============================================
DROP TABLE IF EXISTS `strategies`;
CREATE TABLE IF NOT EXISTS strategies (
    id                INT UNSIGNED    NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    uid               INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '用户ID',
    name              VARCHAR(100)    NOT NULL COMMENT '策略名称',
    description       VARCHAR(500)    NOT NULL DEFAULT '' COMMENT '策略描述',
    logical_op        VARCHAR(10)     NOT NULL DEFAULT 'and' COMMENT '逻辑运算: and/or',
    conditions        TEXT            NOT NULL COMMENT '信号条件JSON数组',

    -- 回测相关
    backtest_count    INT             NOT NULL DEFAULT 0 COMMENT '回测次数',
    last_run_at       DATETIME        COMMENT '最后运行时间',

    -- 回测 — 卖出规则与仓位管理
    exit_rules        JSON            COMMENT '卖出规则集: {stop_loss, take_profit, time_exit, exit_signals, slippage_pct}',
    position_rules    JSON            COMMENT '仓位管理: {max_positions, max_single_pct, allocation}',

    -- 元数据
    is_public         TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '是否公开',
    star_count        INT             NOT NULL DEFAULT 0 COMMENT '收藏数',

    created_at        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    deleted_at        DATETIME        COMMENT '软删除时间',

    PRIMARY KEY (id),
    INDEX idx_uid (uid),
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='选股策略表';
