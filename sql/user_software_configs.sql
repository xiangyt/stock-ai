-- =============================================
-- 用户软件配置表: 保存个人主页中各软件(cookie等)配置
-- 每个用户每个软件一条记录，未配置时前端展示默认值
-- =============================================

CREATE TABLE IF NOT EXISTS user_software_configs (
    id            INT UNSIGNED    NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id       INT UNSIGNED    NOT NULL COMMENT '用户ID',
    software_name VARCHAR(50)     NOT NULL COMMENT '软件标识: eastmoney/ths/ths2/tencentstock',
    display_name  VARCHAR(100)    NOT NULL DEFAULT '' COMMENT '显示名称',
    cookie        TEXT            COMMENT 'Cookie 字符串',
    extra         JSON            COMMENT '扩展配置(JSON)',
    enabled       TINYINT(1)      NOT NULL DEFAULT 1 COMMENT '是否启用: 0=禁用 1=启用',
    created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE INDEX idx_user_software (user_id, software_name),
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='用户软件配置表';
