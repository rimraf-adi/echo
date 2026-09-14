package cli

import (
	"fmt"
	"strings"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <cp-id>",
	Short: "Show details of a specific checkpoint",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		cp, err := core.LoadCheckpoint(ws, args[0])
		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(cp)
			return
		}

		fmt.Printf("Checkpoint:  %s\n", output.Bold(cp.ID))
		if len(cp.Parents) > 0 {
			fmt.Printf("Parents:     %s\n", strings.Join(cp.Parents, ", "))
		}
		fmt.Printf("Tree Hash:   %s\n", cp.TreeHash)
		fmt.Printf("Created:     %s (%s)\n", cp.CreatedAt.Format("2006-01-02 15:04:05 UTC"), output.RelativeTime(cp.CreatedAt))

		if cp.Metadata.Agent != "" {
			fmt.Printf("Agent:       %s\n", output.Cyan(cp.Metadata.Agent))
		}
		if cp.Metadata.Task != "" {
			fmt.Printf("Task:        %s\n", cp.Metadata.Task)
		}
		if cp.Metadata.Message != "" {
			fmt.Printf("Message:     %s\n", cp.Metadata.Message)
		}
		if len(cp.Metadata.Tags) > 0 {
			fmt.Printf("Tags:        %s\n", strings.Join(cp.Metadata.Tags, ", "))
		}

		fmt.Printf("Total Files: %d (%s)\n", cp.Stats.TotalFiles, output.FormatBytes(cp.Stats.TotalSizeBytes))
		fmt.Printf("Blobs:       %d reused, %d new\n", cp.Stats.BlobsReused, cp.Stats.BlobsNew)
		fmt.Println()

		summary := cp.Changeset.Summary()
		fmt.Printf("Changeset: %d added, %d modified, %d deleted\n", summary["added"], summary["modified"], summary["deleted"])

		for _, p := range cp.Changeset.Added {
			fmt.Printf("  %s %s\n", output.Green("added:   "), p)
		}
		for _, p := range cp.Changeset.Modified {
			fmt.Printf("  %s %s\n", output.Yellow("modified:"), p)
		}
		for _, p := range cp.Changeset.Deleted {
			fmt.Printf("  %s %s\n", output.Red("deleted: "), p)
		}
	},
}

func init() {
	RootCmd.AddCommand(showCmd)
}
