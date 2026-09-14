package cli

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
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
		res, err := core.Revert(ws, targetRef, revertDryRun, !revertNoCheckpoint)
		if err != nil {
			HandleError(err)
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
