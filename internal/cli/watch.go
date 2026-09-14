package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/echo-vcs/echo/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	watchDebounceMs int
	watchAgent      string
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch workspace and automatically create checkpoints whenever files change",
	Long: `Watch monitors the workspace for file changes and automatically creates atomic checkpoints.
When an agent bulk-writes or modifies files, Echo waits for disk activity to settle
for the debounce window (default 1000ms) before committing a checkpoint.`,
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		debounce := time.Duration(watchDebounceMs) * time.Millisecond

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			if !FlagQuiet && !FlagJSON {
				fmt.Println("\nStopping watcher...")
			}
			cancel()
		}()

		// Callback whenever an auto-checkpoint is triggered
		onCheckpoint := func(cp *core.Checkpoint) {
			if FlagJSON {
				output.PrintJSON(map[string]any{
					"event":         "auto_checkpoint",
					"checkpoint_id": cp.ID,
					"tree_hash":     cp.TreeHash,
					"changeset":     cp.Changeset,
					"stats":         cp.Stats,
				})
				return
			}

			summary := cp.Changeset.Summary()
			fmt.Printf("%s Auto-checkpoint %s created (%s)\n",
				output.Green("✔"),
				output.Bold(cp.ID),
				output.RelativeTime(cp.CreatedAt),
			)
			fmt.Printf("  Changes: %s added, %s modified, %s deleted\n",
				output.Green(fmt.Sprintf("+%d", summary["added"])),
				output.Yellow(fmt.Sprintf("~%d", summary["modified"])),
				output.Red(fmt.Sprintf("-%d", summary["deleted"])),
			)
		}

		w, err := watcher.NewWatcher(ws, debounce, watchAgent, onCheckpoint)
		if err != nil {
			HandleError(err)
		}
		defer w.Stop()

		// Initial check: if there are already dirty changes, auto-checkpoint them right away
		initialCP, err := core.AutoCheckpointIfDirty(ws, "existing uncheckpointed changes", watchAgent)
		if err == nil && initialCP != nil {
			_ = index.IndexWorkspaceCheckpoint(ws, initialCP)
			onCheckpoint(initialCP)
		}

		if !FlagQuiet && !FlagJSON {
			fmt.Printf("%s Watching %s for file changes (debounce: %dms)...\n",
				output.Cyan("👁"), ws.RootDir, watchDebounceMs)
			fmt.Println("Press Ctrl+C to stop.")
		}

		if err := w.Start(ctx); err != nil {
			HandleError(err)
		}
	},
}

func init() {
	watchCmd.Flags().IntVar(&watchDebounceMs, "debounce", 1000, "Debounce duration in milliseconds before checkpointing")
	watchCmd.Flags().StringVar(&watchAgent, "agent", "auto-watcher", "Agent tag to attach to auto-checkpoints")
	RootCmd.AddCommand(watchCmd)
}
