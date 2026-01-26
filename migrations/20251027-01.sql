-- Migration 20251027-01: Create rtc_room table
-- Description: 创建房间表，用于存储音视频房间信息
-- Created: 2025-10-27

CREATE TABLE IF NOT EXISTS rtc_room (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT '房间ID',
    creator VARCHAR(40) NOT NULL DEFAULT '' COMMENT '房间创建者',
    room_id VARCHAR(40) NOT NULL DEFAULT '' COMMENT '房间ID',
    rtc_type SMALLINT NOT NULL DEFAULT 0 COMMENT '0: 语音, 1: 视频',
    invite_on SMALLINT NOT NULL DEFAULT 0 COMMENT '0: 否, 1: 是',
    status SMALLINT NOT NULL DEFAULT 0 COMMENT '0: 未开始, 1: 进行中, 2: 已结束, 3: 已取消',
    max_participants INT NOT NULL DEFAULT 2 COMMENT '最多参与者数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY uk_room_id (room_id),
    INDEX idx_creator (creator),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='音视频房间表';

