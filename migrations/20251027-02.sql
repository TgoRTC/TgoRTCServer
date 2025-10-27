-- Migration 20251027-02: Create call_participant table
-- Description: 创建参与者表，用于存储房间参与者信息
-- Created: 2025-10-27

CREATE TABLE IF NOT EXISTS call_participant (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT '参与者ID',
    room_id VARCHAR(40) NOT NULL DEFAULT '' COMMENT '房间ID',
    uid VARCHAR(40) NOT NULL DEFAULT '' COMMENT '用户ID',
    status SMALLINT NOT NULL DEFAULT 0 COMMENT '0: 已加入, 1: 已挂断',
    join_time BIGINT COMMENT '加入时间戳(毫秒)',
    leave_time BIGINT COMMENT '离开时间戳(毫秒)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY uk_room_uid (room_id, uid),
    INDEX idx_room_id (room_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='音视频房间参与者表';

