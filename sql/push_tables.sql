-- =============================================
-- 推送配置表: 存储用户的推送机器人配置
-- 支持多渠道: QQ / 企微 / 钉钉 / 飞书
-- =============================================

CREATE TABLE IF NOT EXISTS push_configs (
    id              INT UNSIGNED    NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id         INT UNSIGNED    NOT NULL COMMENT '所属用户ID',
    name            VARCHAR(100)    NOT NULL COMMENT '机器人名称',
    channel         VARCHAR(20)     NOT NULL COMMENT '渠道: qq/wecom/dingtalk/feishu',
    webhook_url     VARCHAR(500)    NOT NULL DEFAULT '' COMMENT 'Webhook地址',
    token           VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'Token/密钥',
    secret          VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '加签密钥(钉钉用)',
    status          TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    INDEX idx_user_id (user_id),
    INDEX idx_channel (channel)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='推送配置表';
