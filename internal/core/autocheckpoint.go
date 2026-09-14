package core

import (
	"fmt"
)

// AutoCheckpointIfDirty automatically creates a checkpoint if there are any uncommitted changes.
// Returns the created checkpoint if one was made, or nil if the working tree was already clean.
func AutoCheckpointIfDirty(ws *Workspace, reason string, agent string) (*Checkpoint, error) {
	if agent == "" {
		agent = "auto"
	}

	opts := CheckpointOpts{
		Agent:   agent,
		Task:    fmt.Sprintf("auto-checkpoint: %s", reason),
		Tags:    []string{"auto"},
		Message: fmt.Sprintf("Automatic safety checkpoint: %s", reason),
	}

	cp, err := CreateCheckpoint(ws, opts)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil // Clean working tree
	}

	return cp, nil
}
