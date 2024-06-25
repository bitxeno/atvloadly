package lib

import "encoding/json"

// Message.Type
const (
	TypeErr = iota
	TypeData
	TypeResize
)

// Message Websocket Communication data format
type Message struct {
	Type int             `json:"t"`
	Data json.RawMessage `json:"d"`
}

// MessageClient Websocket Communication data format
type MessageClient struct {
	Type int         `json:"t"`
	Data interface{} `json:"d"`
}
