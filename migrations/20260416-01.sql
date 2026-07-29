-- Migration 20260416-01: Add call_mode to rtc_room table
-- Description: 添加通话模式字段，支持 AI Agent 通话
-- Created: 2026-04-16

ALTER TABLE rtc_room
ADD COLUMN call_mode TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '通话模式: 0-普通通话, 1-AI通话' AFTER max_participants;
