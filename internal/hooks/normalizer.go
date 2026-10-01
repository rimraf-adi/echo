package hooks

// Canonical Event Types
const (
	EventSessionStarted    = "SessionStarted"
	EventUserPrompt        = "UserPrompt"
	EventAgentResponse     = "AgentResponse"
	EventToolStarted       = "ToolStarted"
	EventToolCompleted     = "ToolCompleted"
	EventFileRead          = "FileRead"
	EventFileChanged       = "FileChanged"
	EventArtifactCreated   = "ArtifactCreated"
	EventSubagentStarted   = "SubagentStarted"
	EventSubagentCompleted = "SubagentCompleted"
	EventSessionEnded      = "SessionEnded"
)

// CanonicalEvent represents the unified schema for all IDE hooks
type CanonicalEvent struct {
	Type           string                 `json:"type"`
	Timestamp      string                 `json:"timestamp"`
	ConversationID string                 `json:"conversation_id"`
	Agent          string                 `json:"agent"` // e.g., "Antigravity", "Cursor"
	Data           map[string]interface{} `json:"data"`
}

// Adapter interface for converting IDE-specific payloads to CanonicalEvent
type Adapter interface {
	Parse(rawPayload []byte) (*CanonicalEvent, error)
}
