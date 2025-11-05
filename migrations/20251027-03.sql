-- Migration 20251027-03: Add source_channel_id index
-- Description: 为 rtc_room 表的 source_channel_id 字段添加索引，提高查询性能
-- Created: 2025-10-27

ALTER TABLE rtc_room ADD INDEX idx_source_channel_id (source_channel_id);

