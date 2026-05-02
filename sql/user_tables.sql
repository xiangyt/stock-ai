-- =============================================
-- 用户表: 存储登录用户信息
-- 密码使用 MD5 哈希存储
-- =============================================

CREATE TABLE IF NOT EXISTS users (
    id            INT UNSIGNED    NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    username      VARCHAR(50)     NOT NULL COMMENT '登录用户名',
    password      VARCHAR(32)     NOT NULL COMMENT '密码(MD5,32位hex)',
    nickname      VARCHAR(50)     NOT NULL DEFAULT '' COMMENT '昵称/显示名',
    avatar        VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '头像URL',
    role          VARCHAR(20)     NOT NULL DEFAULT 'user' COMMENT '角色: user/admin',
    status        TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=正常',
    last_login_at DATETIME       COMMENT '最后登录时间',
    created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE INDEX idx_username (username),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='用户表';

-- 默认管理员账号（密码: admin123）
-- hash = MD5("admin123") = 0192023a7bbd73250516f069df18b500
INSERT IGNORE INTO users (username, password, nickname, role, status)
VALUES ('admin', '0192023a7bbd73250516f069df18b500', '管理员', 'admin', 1);
