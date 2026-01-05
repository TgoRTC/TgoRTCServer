-- Migration 20260104-05: Add device_type to rtc_participant table
-- Description: 添加设备类型字段
-- Created: 2025-12-31

ALTER TABLE rtc_participant 
ADD COLUMN device_type VARCHAR(20) NOT NULL DEFAULT '' COMMENT '设备类型' AFTER uid;
