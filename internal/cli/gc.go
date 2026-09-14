package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var gcDryRun bool

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect unreferenced objects to reclaim disk space",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		startTime := time.Now()

		checkpoints, err := core.ListCheckpoints(ws)
		if err != nil {
			HandleError(err)
		}

		referenced := make(map[string]bool)

		// Collect all referenced tree and blob hashes
		for _, cp := range checkpoints {
			referenced[cp.TreeHash] = true

			treeData, err := ws.Store.ReadTree(cp.TreeHash)
			if err != nil {
				continue
			}
			treeNode, err := merkle.DeserializeTree(treeData)
			if err != nil {
				continue
			}

			_ = merkle.WalkTree(treeNode, ws.Store, func(p string, entry merkle.TreeEntry, isDir bool) error {
				referenced[entry.Hash] = true
				return nil
			})
		}

		allHashes, err := ws.Store.ListObjects()
		if err != nil {
			HandleError(err)
		}

		var unreferenced []string
		var bytesFreed int64

		for _, h := range allHashes {
			if !referenced[h] {
				unreferenced = append(unreferenced, h)
				path, _ := ws.Store.ObjectPath(h)
				if info, err := os.Stat(path); err == nil {
					bytesFreed += info.Size()
				}
				if !gcDryRun {
					_ = ws.Store.DeleteObject(h)
				}
			}
		}

		durationMs := time.Since(startTime).Milliseconds()

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"objects_scanned":  len(allHashes),
				"objects_removed":  len(unreferenced),
				"bytes_freed":      bytesFreed,
				"dry_run":          gcDryRun,
				"duration_ms":      durationMs,
			})
			return
		}

		if gcDryRun {
			fmt.Printf("%s Dry-run GC: %d objects (%s) would be removed (%dms)\n",
				output.Yellow("!"), len(unreferenced), output.FormatBytes(bytesFreed), durationMs)
		} else {
			fmt.Printf("%s Garbage collected %d unreferenced objects (%s freed, %dms)\n",
				output.Green("✔"), len(unreferenced), output.FormatBytes(bytesFreed), durationMs)
		}
	},
}

func init() {
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Identify unreferenced objects without deleting")
	RootCmd.AddCommand(gcCmd)
}
