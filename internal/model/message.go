package model

import "encoding/json"

// Message.Type
const (
	MessageTypeInstall = 1
	MessageType2FA     = 2

	MessageTypePair        = 1
	MessageTypePairConfirm = 2
)

// Message Websocket Communication data format
type Message struct {
	Type int             `json:"t"`
	Data json.RawMessage `json:"d"`
}
