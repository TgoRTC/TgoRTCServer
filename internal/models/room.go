package models

import (
	"time"
)

// Room 房间模型
type Room struct {
	ID                int       `gorm:"primaryKey" json:"id"`
	SourceChannelID   string    `gorm:"column:source_channel_id;size:100;not null;default:''" json:"source_channel_id"`
	SourceChannelType int16     `gorm:"column:source_channel_type;not null;default:0" json:"source_channel_type"`
	Creator           string    `gorm:"column:creator;size:40;not null;default:''" json:"creator"`
	RoomID            string    `gorm:"column:room_id;size:40;not null;default:'';uniqueIndex" json:"room_id"`
	CallType          int16     `gorm:"column:call_type;not null;default:0" json:"call_type"`               // 0: 语音, 1: 视频
	InviteOn          int16     `gorm:"column:invite_on;not null;default:0" json:"invite_on"`               // 0: 否, 1: 是
	Status            int16     `gorm:"column:status;not null;default:0" json:"status"`                     // 0: 未开始, 1: 进行中, 2: 已结束, 3: 已取消
	MaxParticipants   int       `gorm:"column:max_participants;not null;default:2" json:"max_participants"` // 最多参与者数
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Room) TableName() string {
	return "call_room"
}

// RoomStatus 房间状态常量
const (
	RoomStatusNotStarted = 0 // 未开始
	RoomStatusInProgress = 1 // 进行中
	RoomStatusFinished   = 2 // 已结束
	RoomStatusCancelled  = 3 // 已取消
)

// CallType 呼叫类型常量
const (
	CallTypeVoice = 0 // 语音
	CallTypeVideo = 1 // 视频
)

// InviteStatus 邀请状态常量
const (
	InviteDisabled = 0 // 不开启邀请
	InviteEnabled  = 1 // 开启邀请
)

// CreateRoomRequest 创建房间请求
type CreateRoomRequest struct {
	SourceChannelID   string   `json:"source_channel_id" binding:"required"`
	SourceChannelType int      `json:"source_channel_type"`
	Creator           string   `json:"creator" binding:"required"`
	RoomID            string   `json:"room_id"`          // 可选，不传则自动生成 UUID
	CallType          int      `json:"call_type"`        // 0: 语音, 1: 视频
	InviteOn          int      `json:"invite_on"`        // 0: 否, 1: 是
	MaxParticipants   int      `json:"max_participants"` // 最多参与者数，默认 2
	UIDs              []string `json:"uids"`             // 邀请的用户 ID 列表
}

// CreateRoomResponse 创建房间响应
type CreateRoomResponse struct {
	RoomID          string `json:"room_id"`
	Creator         string `json:"creator"`
	Token           string `json:"token"`
	URL             string `json:"url"`
	Status          int16  `json:"status"`
	CreatedAt       string `json:"created_at"` // yyyy-mm-dd hh:mm:ss 格式
	MaxParticipants int    `json:"max_participants"`
	Timeout         int    `json:"timeout"` // 单位：秒
}

// GetRoomResponse 获取房间响应
type GetRoomResponse struct {
	ID                int    `json:"id"`
	SourceChannelID   string `json:"source_channel_id"`
	SourceChannelType int16  `json:"source_channel_type"`
	Creator           string `json:"creator"`
	RoomID            string `json:"room_id"`
	CallType          int16  `json:"call_type"`
	InviteOn          int16  `json:"invite_on"`
	Status            int16  `json:"status"`
	CreatedAt         string `json:"created_at"` // yyyy-mm-dd hh:mm:ss 格式
	UpdatedAt         string `json:"updated_at"` // yyyy-mm-dd hh:mm:ss 格式
}

// UpdateRoomStatusRequest 更新房间状态请求
type UpdateRoomStatusRequest struct {
	Status int `json:"status" binding:"required"`
}
