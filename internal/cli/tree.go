package cli

import (
	"fmt"
	"strings"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	treeAtCP  string
	treeDepth int
	treePath  string
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show directory tree structure at current or historical checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		targetRef := treeAtCP
		if targetRef == "" {
			targetRef = "HEAD"
		}

		targetID, err := core.ResolveRef(ws, targetRef)
		if err != nil {
			HandleError(err)
		}

		cp, err := core.LoadCheckpoint(ws, targetID)
		if err != nil {
			HandleError(err)
		}

		treeData, err := ws.Store.ReadTree(cp.TreeHash)
		if err != nil {
			HandleError(err)
		}

		treeNode, err := merkle.DeserializeTree(treeData)
		if err != nil {
			HandleError(err)
		}

		type treeItem struct {
			Path  string `json:"path"`
			IsDir bool   `json:"is_dir"`
			Size  int64  `json:"size,omitempty"`
		}

		var items []treeItem
		var totalDirs, totalFiles int
		var totalSize int64

		err = merkle.WalkTree(treeNode, ws.Store, func(p string, entry merkle.TreeEntry, isDir bool) error {
			if treePath != "" && !strings.HasPrefix(p, treePath) {
				return nil
			}

			if treeDepth > 0 {
				depth := strings.Count(p, "/") + 1
				if depth > treeDepth {
					return nil
				}
			}

			if isDir {
				totalDirs++
			} else {
				totalFiles++
				totalSize += entry.Size
			}

			items = append(items, treeItem{
				Path:  p,
				IsDir: isDir,
				Size:  entry.Size,
			})
			return nil
		})
		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"checkpoint_id": targetID,
				"tree":          items,
				"total_dirs":    totalDirs,
				"total_files":   totalFiles,
				"total_size":    totalSize,
			})
			return
		}

		fmt.Println(".")
		for _, item := range items {
			indent := strings.Repeat("│   ", strings.Count(item.Path, "/"))
			name := item.Path
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}

			if item.IsDir {
				fmt.Printf("%s├── %s/\n", indent, output.Bold(name))
			} else {
				fmt.Printf("%s├── %s (%s)\n", indent, name, output.FormatBytes(item.Size))
			}
		}
		fmt.Println()
		fmt.Printf("%d directories, %d files, %s total\n", totalDirs, totalFiles, output.FormatBytes(totalSize))
	},
}

func init() {
	treeCmd.Flags().StringVar(&treeAtCP, "at", "", "Checkpoint for tree structure (default HEAD)")
	treeCmd.Flags().IntVarP(&treeDepth, "depth", "d", 0, "Maximum directory depth")
	treeCmd.Flags().StringVar(&treePath, "path", "", "Subdirectory path filter")
	RootCmd.AddCommand(treeCmd)
}
