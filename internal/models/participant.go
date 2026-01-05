package models

import (
	"time"
)

// Participant 参与者模型
type Participant struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	RoomID     string    `gorm:"column:room_id;size:40;not null;default:'';index:idx_room_uid,unique" json:"room_id"`
	UID        string    `gorm:"column:uid;size:40;not null;default:'';index:idx_uid;index:idx_room_uid,unique" json:"uid"`
	DeviceType string    `gorm:"column:device_type;size:20;not null;default:''" json:"device_type"` // 设备类型
	Status     uint8     `gorm:"column:status;not null;default:0" json:"status"`                    // 0-6: 见常量定义
	JoinTime   int64     `gorm:"column:join_time;not null;default:0" json:"join_time"`
	LeaveTime  int64     `gorm:"column:leave_time;not null;default:0" json:"leave_time"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Participant) TableName() string {
	return "rtc_participant"
}

// ParticipantStatus 参与者状态常量
const (
	ParticipantStatusInviting  = 0 // 邀请中
	ParticipantStatusJoined    = 1 // 已加入
	ParticipantStatusRejected  = 2 // 已拒绝
	ParticipantStatusHangup    = 3 // 已挂断
	ParticipantStatusMissed    = 4 // 超时未加入
	ParticipantStatusBusy      = 5 // 通话中未接听
	ParticipantStatusCancelled = 6 // 已取消
)

// JoinRoomRequest 加入房间请求
// room_id 从 URL 参数中获取: POST /api/v1/rooms/:room_id/join
type JoinRoomRequest struct {
	RoomID     string `json:"room_id"`    // 从 URL 参数中设置
	UID        string `json:"uid" binding:"required"`
	DeviceType string `json:"device_type"` // 设备类型
}

// JoinRoomResponse 加入房间响应（别名，保持向后兼容）
type JoinRoomResponse = RoomResp

// LeaveRoomRequest 离开房间请求
// room_id 从 URL 参数中获取: POST /api/v1/rooms/:room_id/leave
type LeaveRoomRequest struct {
	RoomID string `json:"room_id"` // 从 URL 参数中设置
	UID    string `json:"uid" binding:"required"`
}

// InviteParticipantRequest 邀请参与者请求
type InviteParticipantRequest struct {
	RoomID string   `json:"room_id"`
	UIDs   []string `json:"uids" binding:"required"`
}

// GetParticipantsResponse 获取参与者列表响应
type GetParticipantsResponse struct {
	ID         int    `json:"id"`
	RoomID     string `json:"room_id"`
	UID        string `json:"uid"`
	DeviceType string `json:"device_type"`
	Status     int16  `json:"status"`
	JoinTime   int64  `json:"join_time"`
	LeaveTime  int64  `json:"leave_time"`
	CreatedAt  string `json:"created_at"` // yyyy-mm-dd hh:mm:ss 格式
	UpdatedAt  string `json:"updated_at"` // yyyy-mm-dd hh:mm:ss 格式
}

// UpdateParticipantStatusRequest 更新参与者状态请求
type UpdateParticipantStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

// GetUserAvailableRoomsRequest 获取用户可加入的房间列表请求
type GetUserAvailableRoomsRequest struct {
	UID string `json:"uid" binding:"required"`
}

// GetUserAvailableRoomsResponse 获取用户可加入的房间列表响应（RoomResp 数组）
type GetUserAvailableRoomsResponse []RoomResp
