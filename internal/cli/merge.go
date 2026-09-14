package cli

import (
	"fmt"
	"strings"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	mergeStrategy string
	mergeDryRun   bool
)

var mergeCmd = &cobra.Command{
	Use:   "merge <branch>",
	Short: "Merge another branch into the current branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		branchName := args[0]
		strat := core.StrategyTheirs
		if strings.ToLower(mergeStrategy) == "ours" {
			strat = core.StrategyOurs
		}

		if !mergeDryRun {
			// Safety check: auto-save uncommitted changes before merging
			if cp, _ := core.AutoCheckpointIfDirty(ws, fmt.Sprintf("auto-save before merge %s", branchName), "safety-guard"); cp != nil {
				_ = index.IndexWorkspaceCheckpoint(ws, cp)
			}
		}

		res, err := core.Merge(ws, branchName, strat, mergeDryRun)
		if err != nil {
			HandleError(err)
		}

		if res.MergeCheckpoint != "" {
			if mcp, err := core.LoadCheckpoint(ws, res.MergeCheckpoint); err == nil {
				_ = index.IndexWorkspaceCheckpoint(ws, mcp)
			}
		}

		if FlagJSON {
			output.PrintJSON(res)
			return
		}

		if mergeDryRun {
			fmt.Printf("%s Dry-run merge %s into %s (strategy: %s)\n",
				output.Yellow("!"), branchName, res.IntoBranch, res.Strategy)
		} else {
			fmt.Printf("%s Merged branch %s into %s\n", output.Green("✔"), output.Bold(branchName), output.Bold(res.IntoBranch))
			fmt.Printf("  Merge checkpoint: %s\n", output.Bold(res.MergeCheckpoint))
		}

		if len(res.Conflicts) > 0 {
			fmt.Printf("  %s %d conflicts resolved (%s):\n", output.Yellow("!"), len(res.Conflicts), res.Strategy)
			for _, c := range res.Conflicts {
				fmt.Printf("    %s (%s)\n", c.Path, c.Resolution)
			}
		}

		summary := res.Changes.Summary()
		fmt.Printf("  Changes: %s added, %s modified, %s deleted\n",
			output.Green(fmt.Sprintf("+%d", summary["added"])),
			output.Yellow(fmt.Sprintf("~%d", summary["modified"])),
			output.Red(fmt.Sprintf("-%d", summary["deleted"])),
		)
	},
}

func init() {
	mergeCmd.Flags().StringVar(&mergeStrategy, "strategy", "theirs", "Conflict resolution strategy: theirs, ours")
	mergeCmd.Flags().BoolVar(&mergeDryRun, "dry-run", false, "Preview merge without applying")
	RootCmd.AddCommand(mergeCmd)
}
