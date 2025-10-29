package models

// WebhookEvent LiveKit webhook 事件
type WebhookEvent struct {
	Event     string      `json:"event"`
	ID        string      `json:"id"`
	CreatedAt int64       `json:"createdAt"`
	Room      *RoomInfo   `json:"room,omitempty"`
	Participant *ParticipantInfo `json:"participant,omitempty"`
	Track     *TrackInfo  `json:"track,omitempty"`
	EgressInfo *EgressInfo `json:"egressInfo,omitempty"`
	IngressInfo *IngressInfo `json:"ingressInfo,omitempty"`
}

// RoomInfo 房间信息
type RoomInfo struct {
	SID       string `json:"sid"`
	Name      string `json:"name"`
	EmptyTimeout int32 `json:"emptyTimeout"`
	MaxParticipants int32 `json:"maxParticipants"`
	CreationTime int64 `json:"creationTime"`
	Metadata  string `json:"metadata"`
	NumParticipants int32 `json:"numParticipants"`
}

// ParticipantInfo 参与者信息
type ParticipantInfo struct {
	SID      string `json:"sid"`
	Identity string `json:"identity"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Metadata string `json:"metadata"`
	JoinedAt int64  `json:"joinedAt"`
}

// TrackInfo 轨道信息
type TrackInfo struct {
	SID      string `json:"sid"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Muted    bool   `json:"muted"`
	Width    uint32 `json:"width"`
	Height   uint32 `json:"height"`
	Bitrate  uint64 `json:"bitrate"`
	Codec    string `json:"codec"`
}

// EgressInfo 导出信息
type EgressInfo struct {
	EgressID string `json:"egressId"`
	RoomID   string `json:"roomId"`
	RoomName string `json:"roomName"`
	Status   string `json:"status"`
	StartedAt int64 `json:"startedAt"`
	UpdatedAt int64 `json:"updatedAt"`
	EndedAt   int64 `json:"endedAt"`
	Error    string `json:"error"`
}

// IngressInfo 导入信息
type IngressInfo struct {
	IngressID string `json:"ingressId"`
	StreamKey string `json:"streamKey"`
	URL       string `json:"url"`
	InputType string `json:"inputType"`
	Codec     string `json:"codec"`
	State     string `json:"state"`
	Error     string `json:"error"`
}

// WebhookEventType webhook 事件类型常量
const (
	WebhookEventRoomStarted              = "room_started"
	WebhookEventRoomFinished             = "room_finished"
	WebhookEventParticipantJoined        = "participant_joined"
	WebhookEventParticipantLeft          = "participant_left"
	WebhookEventParticipantConnectionAborted = "participant_connection_aborted"
	WebhookEventTrackPublished           = "track_published"
	WebhookEventTrackUnpublished         = "track_unpublished"
	WebhookEventEgressStarted            = "egress_started"
	WebhookEventEgressUpdated            = "egress_updated"
	WebhookEventEgressEnded              = "egress_ended"
	WebhookEventIngressStarted           = "ingress_started"
	WebhookEventIngressEnded             = "ingress_ended"
)

