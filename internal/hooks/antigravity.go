package hooks

import (
	"encoding/json"
	"fmt"
)

// AntigravityAdapter parses Antigravity IDE payloads
type AntigravityAdapter struct{}

// Parse converts an empirical Antigravity payload to a CanonicalEvent
func (a *AntigravityAdapter) Parse(rawPayload []byte) (*CanonicalEvent, error) {
	// TODO: We need the empirical JSON schema from the IDE!
	// The prompt specified it contains: conversationId, workspacePaths, transcriptPath, artifactDirectoryPath, modelName
	
	var payload map[string]interface{}
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, err
	}

	// We will fill this in fully once we inspect the actual hook output.
	event := &CanonicalEvent{
		Agent: "Antigravity",
		Data:  payload,
	}
	
	if cid, ok := payload["conversationId"].(string); ok {
		event.ConversationID = cid
	}
	
	return event, nil
}
