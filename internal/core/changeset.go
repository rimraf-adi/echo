package core

import (
	"sort"

	"github.com/echo-vcs/echo/internal/merkle"
)

// Changeset groups changed file paths by operation type
type Changeset struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

// GroupDeltas groups a slice of merkle.Delta into a sorted Changeset
func GroupDeltas(deltas []merkle.Delta) Changeset {
	cs := Changeset{
		Added:    make([]string, 0),
		Modified: make([]string, 0),
		Deleted:  make([]string, 0),
	}

	for _, d := range deltas {
		switch d.Type {
		case merkle.DeltaAdded:
			cs.Added = append(cs.Added, d.Path)
		case merkle.DeltaModified:
			cs.Modified = append(cs.Modified, d.Path)
		case merkle.DeltaDeleted:
			cs.Deleted = append(cs.Deleted, d.Path)
		}
	}

	sort.Strings(cs.Added)
	sort.Strings(cs.Modified)
	sort.Strings(cs.Deleted)

	return cs
}

// TotalChanges returns the total number of affected files
func (cs *Changeset) TotalChanges() int {
	return len(cs.Added) + len(cs.Modified) + len(cs.Deleted)
}

// Summary returns a map of counts by operation
func (cs *Changeset) Summary() map[string]int {
	return map[string]int{
		"added":    len(cs.Added),
		"modified": len(cs.Modified),
		"deleted":  len(cs.Deleted),
	}
}
