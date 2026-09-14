package cli

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var branchFrom string

var branchCmd = &cobra.Command{
	Use:   "branch <name>",
	Short: "Create a new branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		name := args[0]
		if err := core.CreateBranch(ws, name, branchFrom); err != nil {
			HandleError(err)
		}

		targetID, _ := core.GetRef(ws, name)
		if FlagJSON {
			output.PrintJSON(map[string]any{
				"branch":        name,
				"checkpoint_id": targetID,
				"status":        "created",
			})
			return
		}

		if !FlagQuiet {
			fmt.Printf("%s Created branch %s pointing to %s\n", output.Green("✔"), output.Bold(name), targetID)
		}
	},
}

var switchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to a different branch and update working directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		name := args[0]
		// Safety check: auto-save uncommitted changes before switching
		if cp, _ := core.AutoCheckpointIfDirty(ws, fmt.Sprintf("auto-save before switch to %s", name), "safety-guard"); cp != nil {
			_ = index.IndexWorkspaceCheckpoint(ws, cp)
		}

		if err := core.SwitchBranch(ws, name); err != nil {
			HandleError(err)
		}

		targetID, _ := core.GetRef(ws, name)
		if FlagJSON {
			output.PrintJSON(map[string]any{
				"branch":        name,
				"checkpoint_id": targetID,
				"status":        "switched",
			})
			return
		}

		if !FlagQuiet {
			fmt.Printf("%s Switched to branch %s (%s)\n", output.Green("✔"), output.Bold(name), targetID)
		}
	},
}

var branchesCmd = &cobra.Command{
	Use:   "branches",
	Short: "List all branches",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		branches, err := core.ListBranches(ws)
		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"branches": branches,
			})
			return
		}

		for _, b := range branches {
			marker := "  "
			if b.IsCurrent {
				marker = "* "
			}
			timeStr := ""
			if !b.CreatedAt.IsZero() {
				timeStr = fmt.Sprintf("(%s)", output.RelativeTime(b.CreatedAt))
			}
			if b.IsCurrent {
				fmt.Printf("%s%-20s %s  %s\n", output.Green(marker), output.Bold(b.Name), b.CheckpointID, timeStr)
			} else {
				fmt.Printf("%s%-20s %s  %s\n", marker, b.Name, b.CheckpointID, timeStr)
			}
		}
	},
}

func init() {
	branchCmd.Flags().StringVar(&branchFrom, "from", "", "Checkpoint or branch to branch from (default HEAD)")
	RootCmd.AddCommand(branchCmd)
	RootCmd.AddCommand(switchCmd)
	RootCmd.AddCommand(branchesCmd)
}
