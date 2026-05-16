-- =============================================
-- 持仓管理模块 DDL
-- 包含: positions(持仓表), position_trades(交易记录表), users表扩展字段
-- =============================================

-- 1. 持仓表
DROP TABLE IF EXISTS `positions`;
CREATE TABLE `positions` (
    `id`               INT UNSIGNED   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `uid`              INT UNSIGNED   NOT NULL DEFAULT 0 COMMENT '用户ID',
    `stock_code`       CHAR(6)        NOT NULL          COMMENT '股票代码(6位数字)',
    `quantity`         INT UNSIGNED   NOT NULL DEFAULT 0 COMMENT '当前持仓数量(股)',
    `avg_cost`         DECIMAL(14,4)  NOT NULL DEFAULT 0.0000 COMMENT '平均成本价',
    `status`           VARCHAR(20)    NOT NULL DEFAULT 'holding' COMMENT '状态: holding=持有中, closed=已清仓',
    `note`             VARCHAR(500)   NOT NULL DEFAULT ''  COMMENT '备注',
    `created_at`       DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`       DATETIME       NULL              COMMENT '软删除时间',

    PRIMARY KEY (`id`),
    INDEX `idx_uid`        (`uid`),
    INDEX `idx_stock_code` (`stock_code`),
    INDEX `idx_status`     (`status`),
    INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='持仓表';

-- 2. 交易记录表
DROP TABLE IF EXISTS `position_trades`;
CREATE TABLE `position_trades` (
    `id`            INT UNSIGNED   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `position_id`   INT UNSIGNED   NOT NULL          COMMENT '关联持仓ID',
    `uid`           INT UNSIGNED   NOT NULL DEFAULT 0 COMMENT '用户ID',
    `trade_type`    TINYINT        NOT NULL          COMMENT '交易类型: 1=买入 2=卖出',
    `quantity`      INT UNSIGNED   NOT NULL          COMMENT '交易数量(股)',
    `price`         DECIMAL(14,4)  NOT NULL          COMMENT '成交价格',
    `amount`        DECIMAL(20,4)  NOT NULL DEFAULT 0.0000 COMMENT '成交金额(不含手续费)',
    `commission`     DECIMAL(14,4)  NOT NULL DEFAULT 0.0000 COMMENT '手续费',
    `trade_date`    DATE           NOT NULL          COMMENT '交易日期',
    `note`          VARCHAR(500)   NOT NULL DEFAULT ''  COMMENT '备注',
    `created_at`    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (`id`),
    INDEX `idx_position_id` (`position_id`),
    INDEX `idx_uid`          (`uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='交易记录表';

-- 3. 用户表 — 新增交易配置字段（幂等执行，忽略列已存在的错误）
ALTER TABLE `users`
    ADD COLUMN IF NOT EXISTS `commission_rate`  DECIMAL(8,4)  NOT NULL DEFAULT 0.00025 COMMENT '交易手续费率(默认万分之2.5)';
-- 注意：MySQL 5.7 不支持 ADD COLUMN IF NOT EXISTS，如需兼容可改用存储过程或手动执行
-- ALTER TABLE `users`
--     ADD COLUMN `commission_rate`  DECIMAL(8,4)  NOT NULL DEFAULT 0.00025 COMMENT '交易手续费率(默认万分之2.5)',
--     ADD COLUMN `min_commission`   TINYINT(1)     NOT NULL DEFAULT 1      COMMENT '最低手续费免五: 0=免五 1=不免五';
