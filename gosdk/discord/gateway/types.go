package gateway

import "encoding/json"

// OpCode defines the gateway message operation codes used by Discord.
type OpCode int

const (
	OpCodeDispatch            OpCode = 0
	OpCodeHeartbeat           OpCode = 1
	OpCodeIdentify            OpCode = 2
	OpCodePresenceUpdate      OpCode = 3
	OpCodeVoiceStateUpdate    OpCode = 4
	OpCodeResume              OpCode = 6
	OpCodeReconnect           OpCode = 7
	OpCodeRequestGuildMembers OpCode = 8
	OpCodeInvalidSession      OpCode = 9
	OpCodeHello               OpCode = 10
	OpCodeHeartbeatAck        OpCode = 11
)

// Payload represents the generic envelope sent over the gateway.
type Payload struct {
	Op OpCode          `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int             `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// IdentifyProperties describes the identifying metadata Discord expects.
type IdentifyProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

// IdentifyPayload is sent when the client first connects to the gateway.
type IdentifyPayload struct {
	Token      string             `json:"token"`
	Properties IdentifyProperties `json:"properties"`
	Compress   bool               `json:"compress,omitempty"`
	Intents    int                `json:"intents"`
	Shard      []int              `json:"shard,omitempty"`
}

// ResumePayload is used when resuming an existing session.
type ResumePayload struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}
