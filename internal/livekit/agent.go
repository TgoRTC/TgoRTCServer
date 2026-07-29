package livekit

import (
	"context"
	"fmt"
	"time"

	"tgo-rtc-server/internal/config"

	lkproto "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// AgentDispatcher LiveKit Agent 调度器
type AgentDispatcher struct {
	client    *lksdk.AgentDispatchClient
	agentName string
}

// NewAgentDispatcher 创建 Agent 调度器
func NewAgentDispatcher(cfg *config.Config) *AgentDispatcher {
	client := lksdk.NewAgentDispatchServiceClient(cfg.LiveKitClientURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
	return &AgentDispatcher{
		client:    client,
		agentName: cfg.LiveKitAgentName,
	}
}

// Dispatch 调度 AI Agent 加入指定房间
func (d *AgentDispatcher) Dispatch(roomID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := d.client.CreateDispatch(ctx, &lkproto.CreateAgentDispatchRequest{
		Room:      roomID,
		AgentName: d.agentName,
	})
	if err != nil {
		return fmt.Errorf("dispatch agent failed: %w", err)
	}

	fmt.Printf("[AgentDispatcher] dispatch 成功: room=%s, agent=%s, dispatch_id=%s\n",
		roomID, d.agentName, resp.GetId())
	return nil
}
