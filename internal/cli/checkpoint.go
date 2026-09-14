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
	cpAgent   string
	cpTask    string
	cpModel   string
	cpTags    []string
	cpMeta    []string
	cpMessage string
)

var checkpointCmd = &cobra.Command{
	Use:     "checkpoint",
	Aliases: []string{"cp"},
	Short:   "Capture current workspace state as an immutable checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		customMeta := make(map[string]any)
		for _, m := range cpMeta {
			parts := strings.SplitN(m, "=", 2)
			if len(parts) == 2 {
				customMeta[parts[0]] = parts[1]
			}
		}

		opts := core.CheckpointOpts{
			Agent:   cpAgent,
			Task:    cpTask,
			Model:   cpModel,
			Tags:    cpTags,
			Custom:  customMeta,
			Message: cpMessage,
		}

		cp, err := core.CreateCheckpoint(ws, opts)
		if err != nil {
			HandleError(err)
		}

		if cp == nil {
			if FlagJSON {
				output.PrintJSON(map[string]any{
					"status":  "clean",
					"message": "nothing to checkpoint, working tree clean",
				})
			} else if !FlagQuiet {
				fmt.Println("nothing to checkpoint, working tree clean")
			}
			return
		}

		// Update SQLite index
		_ = index.IndexWorkspaceCheckpoint(ws, cp)

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"checkpoint_id": cp.ID,
				"parents":       cp.Parents,
				"tree_hash":     cp.TreeHash,
				"changeset":     cp.Changeset,
				"stats":         cp.Stats,
			})
			return
		}

		if !FlagQuiet {
			fmt.Printf("%s Checkpoint %s created (%dms)\n", output.Green("✔"), output.Bold(cp.ID), cp.Stats.DurationMs)
			summary := cp.Changeset.Summary()
			fmt.Printf("  Changes: %s added, %s modified, %s deleted\n",
				output.Green(fmt.Sprintf("+%d", summary["added"])),
				output.Yellow(fmt.Sprintf("~%d", summary["modified"])),
				output.Red(fmt.Sprintf("-%d", summary["deleted"])),
			)
			if cp.Metadata.Agent != "" {
				fmt.Printf("  Agent:   %s\n", cp.Metadata.Agent)
			}
			if cp.Metadata.Task != "" {
				fmt.Printf("  Task:    %s\n", cp.Metadata.Task)
			}
		}
	},
}

func init() {
	checkpointCmd.Flags().StringVar(&cpAgent, "agent", "", "Agent identifier")
	checkpointCmd.Flags().StringVar(&cpTask, "task", "", "Task description")
	checkpointCmd.Flags().StringVar(&cpModel, "model", "", "Model identifier")
	checkpointCmd.Flags().StringSliceVar(&cpTags, "tag", nil, "Tags associated with this checkpoint")
	checkpointCmd.Flags().StringSliceVar(&cpMeta, "meta", nil, "Custom metadata in key=value format")
	checkpointCmd.Flags().StringVarP(&cpMessage, "message", "m", "", "Human-readable commit message")
	RootCmd.AddCommand(checkpointCmd)
}
