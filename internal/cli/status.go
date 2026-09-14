package cli

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current workspace status and uncheckpointed changes",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		headBranch, err := core.ReadHead(ws)
		if err != nil {
			HandleError(err)
		}

		headID, _ := core.GetRef(ws, headBranch)
		var headCP *core.Checkpoint
		if headID != "" {
			headCP, _ = core.LoadCheckpoint(ws, headID)
		}

		diffRes, err := core.DiffWorking(ws, "HEAD")
		if err != nil {
			HandleError(err)
		}

		var added, modified, deleted []string
		for _, f := range diffRes.Files {
			switch f.Type {
			case "added":
				added = append(added, f.Path)
			case "modified":
				modified = append(modified, f.Path)
			case "deleted":
				deleted = append(deleted, f.Path)
			}
		}

		changes := core.Changeset{
			Added:    added,
			Modified: modified,
			Deleted:  deleted,
		}

		if FlagJSON {
			var headCreatedAt string
			if headCP != nil {
				headCreatedAt = headCP.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			output.PrintJSON(map[string]any{
				"branch":          headBranch,
				"head":            headID,
				"head_created_at": headCreatedAt,
				"changes":         changes,
				"total_changes":   changes.TotalChanges(),
			})
			return
		}

		fmt.Printf("Branch: %s\n", output.Bold(headBranch))
		if headCP != nil {
			agentInfo := ""
			if headCP.Metadata.Agent != "" {
				agentInfo = fmt.Sprintf(", agent: %s", headCP.Metadata.Agent)
			}
			fmt.Printf("HEAD:   %s (%s%s)\n", headCP.ID, output.RelativeTime(headCP.CreatedAt), agentInfo)
		} else {
			fmt.Println("HEAD:   (no checkpoints yet)")
		}
		fmt.Println()

		if changes.TotalChanges() == 0 {
			fmt.Println("nothing to checkpoint, working tree clean")
			return
		}

		fmt.Println("Changes not checkpointed:")
		for _, p := range added {
			fmt.Printf("  %s %s\n", output.Green("added:   "), p)
		}
		for _, p := range modified {
			fmt.Printf("  %s %s\n", output.Yellow("modified:"), p)
		}
		for _, p := range deleted {
			fmt.Printf("  %s %s\n", output.Red("deleted: "), p)
		}
		fmt.Printf("\n%d files changed\n", changes.TotalChanges())
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
