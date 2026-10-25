package model

// PairingDiscoveryMessage is streamed to the pair page while it discovers the
// manual-pairing port of a manually entered IP. It is JSON-encoded on the same
// websocket as the raw pairing log; the page tells them apart by a leading '{'.
type PairingDiscoveryMessage struct {
	Kind    string `json:"kind"` // always "discover"
	Stage   string `json:"stage,omitempty"`
	Percent int    `json:"percent,omitempty"`
	Port    uint16 `json:"port,omitempty"`
	Name    string `json:"name,omitempty"`
	Model   string `json:"model,omitempty"`
	Message string `json:"message,omitempty"`
}
