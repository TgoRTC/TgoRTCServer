package models

import (
	"time"
)

// Room 房间模型
type Room struct {
	ID                int       `gorm:"primaryKey" json:"id"`
	SourceChannelID   string    `gorm:"column:source_channel_id;size:100;not null;default:''" json:"source_channel_id"`
	SourceChannelType uint8     `gorm:"column:source_channel_type;not null;default:0" json:"source_channel_type"`
	Creator           string    `gorm:"column:creator;size:40;not null;default:''" json:"creator"`
	RoomID            string    `gorm:"column:room_id;size:40;not null;default:'';uniqueIndex" json:"room_id"`
	RTCType           uint8     `gorm:"column:rtc_type;not null;default:0" json:"rtc_type"`                 // 0: 语音, 1: 视频
	InviteOn          uint8     `gorm:"column:invite_on;not null;default:0" json:"invite_on"`               // 0: 否, 1: 是
	Status            uint8     `gorm:"column:status;not null;default:0" json:"status"`                     // 0: 未开始, 1: 进行中, 2: 已结束, 3: 已取消
	MaxParticipants   int       `gorm:"column:max_participants;not null;default:2" json:"max_participants"` // 最多参与者数
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Room) TableName() string {
	return "rtc_room"
}

// RoomStatus 房间状态常量
const (
	RoomStatusNotStarted = 0 // 未开始
	RoomStatusInProgress = 1 // 进行中
	RoomStatusFinished   = 2 // 已结束
	RoomStatusCancelled  = 3 // 已取消
	RoomStatusRejected   = 4 // 已拒绝
	RoomStatusBusy       = 5 // 通话中未接听
	RoomStatusMissed     = 6 // 超时未加入
)

// RTCType 呼叫类型常量
const (
	RTCTypeVoice = 0 // 语音
	RTCTypeVideo = 1 // 视频
)

// InviteStatus 邀请状态常量
const (
	InviteDisabled = 0 // 不开启邀请
	InviteEnabled  = 1 // 开启邀请
)

// CreateRoomRequest 创建房间请求
type CreateRoomRequest struct {
	SourceChannelID   string   `json:"source_channel_id" binding:"required"`
	SourceChannelType uint8    `json:"source_channel_type"`
	Creator           string   `json:"creator" binding:"required"`
	RoomID            string   `json:"room_id"`          // 可选，不传则自动生成 UUID
	RTCType           uint8    `json:"rtc_type"`         // 0: 语音, 1: 视频
	InviteOn          uint8    `json:"invite_on"`        // 0: 否, 1: 是
	MaxParticipants   int      `json:"max_participants"` // 最多参与者数，默认 2
	UIDs              []string `json:"uids"`             // 邀请的用户 ID 列表
}

// RoomResp 房间响应（创建房间和加入房间共用）
type RoomResp struct {
	SourceChannelID   string   `json:"source_channel_id" binding:"required"`
	SourceChannelType uint8    `json:"source_channel_type"`
	RoomID            string   `json:"room_id"`
	Creator           string   `json:"creator"`
	Token             string   `json:"token"`
	URL               string   `json:"url"`
	Status            uint8    `json:"status"`
	CreatedAt         string   `json:"created_at"` // yyyy-mm-dd hh:mm:ss 格式
	MaxParticipants   int      `json:"max_participants"`
	Timeout           int      `json:"timeout"`  // 单位：秒
	UIDs              []string `json:"uids"`     // 参与者uids
	RTCType           uint8    `json:"rtc_type"` // 0: 语音, 1: 视频
}

// CreateRoomResponse 创建房间响应（别名，保持向后兼容）
type CreateRoomResponse = RoomResp

// UpdateRoomStatusRequest 更新房间状态请求
type UpdateRoomStatusRequest struct {
	Status int `json:"status" binding:"required"`
}
