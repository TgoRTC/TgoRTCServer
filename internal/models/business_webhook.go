package models

import "time"

// BusinessWebhookEvent 业务 webhook 事件
type BusinessWebhookEvent struct {
	EventType   string      `json:"event_type"`   // 事件类型
	EventID     string      `json:"event_id"`     // 事件 ID（UUID）
	Timestamp   int64       `json:"timestamp"`    // 事件时间戳（秒）
	Data        interface{} `json:"data"`         // 事件数据
	Retry       int         `json:"retry"`        // 重试次数
	RetryAfter  int         `json:"retry_after"`  // 重试延迟（秒）
}

// 业务事件类型常量
const (
	// 房间事件
	BusinessEventRoomCreated   = "room.created"   // 房间已创建
	BusinessEventRoomStarted   = "room.started"   // 房间已开始
	BusinessEventRoomFinished  = "room.finished"  // 房间已结束
	BusinessEventRoomCancelled = "room.cancelled" // 房间已取消

	// 参与者事件
	BusinessEventParticipantInvited      = "participant.invited"       // 参与者已邀请
	BusinessEventParticipantJoined       = "participant.joined"        // 参与者已加入
	BusinessEventParticipantLeft         = "participant.left"          // 参与者已离开
	BusinessEventParticipantRejected     = "participant.rejected"      // 参与者已拒绝
	BusinessEventParticipantTimeout      = "participant.timeout"       // 参与者已超时
	BusinessEventParticipantMissed       = "participant.missed"        // 参与者已错过
	BusinessEventParticipantCancelled    = "participant.cancelled"     // 参与者已取消
)

// RoomEventData 房间事件数据
type RoomEventData struct {
	RoomID          string    `json:"room_id"`
	Creator         string    `json:"creator"`
	CallType        int       `json:"call_type"`        // 0: 语音, 1: 视频
	InviteOn        int       `json:"invite_on"`        // 0: 否, 1: 是
	Status          int       `json:"status"`           // 房间状态
	MaxParticipants int       `json:"max_participants"` // 最大参与者数
	CreatedAt       int64     `json:"created_at"`
	UpdatedAt       int64     `json:"updated_at"`
}

// ParticipantEventData 参与者事件数据
type ParticipantEventData struct {
	RoomID    string `json:"room_id"`
	UID       string `json:"uid"`
	Status    int    `json:"status"`     // 参与者状态
	JoinTime  int64  `json:"join_time"`  // 加入时间戳（秒）
	LeaveTime int64  `json:"leave_time"` // 离开时间戳（秒）
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// BusinessWebhookRequest 业务 webhook 请求
type BusinessWebhookRequest struct {
	EventType  string      `json:"event_type"`
	EventID    string      `json:"event_id"`
	Timestamp  int64       `json:"timestamp"`
	Data       interface{} `json:"data"`
	Retry      int         `json:"retry"`
	RetryAfter int         `json:"retry_after"`
}

// BusinessWebhookResponse 业务 webhook 响应
type BusinessWebhookResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// BusinessWebhookLog 业务 webhook 日志
type BusinessWebhookLog struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	EventType string    `json:"event_type"`
	EventID   string    `json:"event_id"`
	URL       string    `json:"url"`
	Status    int       `json:"status"`        // HTTP 状态码
	Request   string    `json:"request"`       // 请求体
	Response  string    `json:"response"`      // 响应体
	Error     string    `json:"error"`         // 错误信息
	Retry     int       `json:"retry"`         // 重试次数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BusinessWebhookLog) TableName() string {
	return "business_webhook_log"
}

