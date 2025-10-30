-- 创建业务 webhook 日志表
CREATE TABLE IF NOT EXISTS business_webhook_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '日志ID',
    event_type VARCHAR(100) NOT NULL DEFAULT '' COMMENT '事件类型',
    event_id VARCHAR(100) NOT NULL DEFAULT '' COMMENT '事件ID',
    url VARCHAR(500) NOT NULL DEFAULT '' COMMENT 'Webhook URL',
    status INT NOT NULL DEFAULT 0 COMMENT 'HTTP 状态码',
    request LONGTEXT COMMENT '请求体',
    response LONGTEXT COMMENT '响应体',
    error VARCHAR(500) COMMENT '错误信息',
    retry INT NOT NULL DEFAULT 0 COMMENT '重试次数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_event_id (event_id),
    INDEX idx_event_type (event_type),
    INDEX idx_url (url),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='业务 webhook 日志表';

