package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var initIgnorePath string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Echo tracking in the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		dir := FlagDir
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				HandleError(err)
			}
		}

		ws, cp, err := core.Init(dir, initIgnorePath)
		if err != nil {
			HandleError(err)
		}

		// Initialize SQLite index
		dbPath := filepath.Join(ws.EchoDir, "index.db")
		idx, err := index.OpenIndex(dbPath)
		if err == nil {
			_ = idx.Rebuild(ws)
			_ = idx.Close()
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"workspace_id":          ws.Config.WorkspaceID,
				"initial_checkpoint":    cp.ID,
				"files_tracked":         cp.Stats.TotalFiles,
				"total_size_bytes":      cp.Stats.TotalSizeBytes,
				"tree_hash":             cp.TreeHash,
			})
			return
		}

		if !FlagQuiet {
			fmt.Printf("%s Initialized Echo workspace (%s)\n", output.Green("✔"), ws.Config.WorkspaceID)
			fmt.Printf("  Initial checkpoint: %s\n", output.Bold(cp.ID))
			fmt.Printf("  Tracked files:      %d (%s)\n", cp.Stats.TotalFiles, output.FormatBytes(cp.Stats.TotalSizeBytes))
			fmt.Printf("  Default branch:     %s\n", ws.Config.DefaultBranch)
		}
	},
}

func init() {
	initCmd.Flags().StringVar(&initIgnorePath, "ignore", "", "Path to custom ignore file to copy as .echo/ignore")
	RootCmd.AddCommand(initCmd)
}
