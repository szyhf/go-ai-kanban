package proxy

// WebRTCConfig holds configuration for WebRTC peer connections.
type WebRTCConfig struct {
	STUNServers []string
}

// DefaultWebRTCConfig returns the default WebRTC configuration.
func DefaultWebRTCConfig() WebRTCConfig {
	return WebRTCConfig{
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
		},
	}
}

// DataChannelMessage is the union type for messages sent over a WebRTC
// data channel. It mirrors the frontend TypeScript DataChannelMessage.
type DataChannelMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload,omitempty"`
	Seq     int64  `json:"seq,omitempty"`
}

// SDPMessage represents a WebRTC SDP offer or answer for signaling.
type SDPMessage struct {
	Type string `json:"type"` // "offer" or "answer"
	SDP  string `json:"sdp"`
}

// ICEMessage represents a WebRTC ICE candidate for signaling.
type ICEMessage struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid,omitempty"`
	SDPMLineIndex *int   `json:"sdpMLineIndex,omitempty"`
}

// RelayChannelName is the name of the WebRTC data channel used for relaying.
const RelayChannelName = "relay"
