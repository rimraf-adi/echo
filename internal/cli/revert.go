package cli

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	revertDryRun       bool
	revertNoCheckpoint bool
)

var revertCmd = &cobra.Command{
	Use:   "revert <cp-id>",
	Short: "Revert workspace to a previous checkpoint state",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		targetRef := args[0]
		if !revertDryRun {
			// Safety check: auto-save uncommitted changes before reverting
			if cp, _ := core.AutoCheckpointIfDirty(ws, fmt.Sprintf("auto-save before revert to %s", targetRef), "safety-guard"); cp != nil {
				_ = index.IndexWorkspaceCheckpoint(ws, cp)
			}
		}

		res, err := core.Revert(ws, targetRef, revertDryRun, !revertNoCheckpoint)
		if err != nil {
			HandleError(err)
		}

		if res.RevertCheckpoint != "" {
			if rcp, err := core.LoadCheckpoint(ws, res.RevertCheckpoint); err == nil {
				_ = index.IndexWorkspaceCheckpoint(ws, rcp)
			}
		}

		if FlagJSON {
			output.PrintJSON(res)
			return
		}

		if revertDryRun {
			fmt.Printf("%s Dry-run revert to %s\n", output.Yellow("!"), res.RevertedTo)
		} else {
			fmt.Printf("%s Reverted workspace to %s\n", output.Green("✔"), output.Bold(res.RevertedTo))
			if res.RevertCheckpoint != "" {
				fmt.Printf("  Created revert checkpoint: %s\n", output.Bold(res.RevertCheckpoint))
			}
		}

		for _, p := range res.ChangesApplied.Added {
			fmt.Printf("  %s %s\n", output.Green("restored:"), p)
		}
		for _, p := range res.ChangesApplied.Modified {
			fmt.Printf("  %s %s\n", output.Yellow("modified:"), p)
		}
		for _, p := range res.ChangesApplied.Deleted {
			fmt.Printf("  %s %s\n", output.Red("deleted: "), p)
		}
	},
}

func init() {
	revertCmd.Flags().BoolVar(&revertDryRun, "dry-run", false, "Preview changes without applying")
	revertCmd.Flags().BoolVar(&revertNoCheckpoint, "no-checkpoint", false, "Do not create a revert checkpoint")
	RootCmd.AddCommand(revertCmd)
}
