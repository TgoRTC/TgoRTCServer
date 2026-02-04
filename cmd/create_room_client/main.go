package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type CreateRoomRequest struct {
	Creator string `json:"creator"`
	RoomID            string   `json:"room_id"`
	RTCType           int      `json:"rtc_type"`
	InviteOn          int      `json:"invite_on"`
	MaxParticipants   int      `json:"max_participants"`
	UIDs              []string `json:"uids"`
}

func main() {
	url := os.Getenv("ROOM_API_URL")
	if url == "" {
		url = "http://localhost:8080/api/v1/rooms"
	}

	payload := CreateRoomRequest{
		Creator: "user001",
		RTCType:           1,
		InviteOn:          1,
		MaxParticipants:   4,
		UIDs:              []string{"user002", "user003"},
	}

	b, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		writeOut(fmt.Sprintf("HTTP request error: %v\n", err))
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	writeOut(fmt.Sprintf("HTTP %d\n%s\n", resp.StatusCode, string(body)))
}

func writeOut(s string) {
	outDir := filepath.Join("test-output")
	_ = os.MkdirAll(outDir, 0o755)
	fn := filepath.Join(outDir, "create_room_response.txt")
	_ = os.WriteFile(fn, []byte(s), 0o644)
	fmt.Print(s)
}
