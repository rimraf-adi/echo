package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	filesAtCP   string
	filesPattern string
	filesSort    string
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "List tracked files at current or historical checkpoint",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		targetRef := filesAtCP
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

		fileMap, err := merkle.FlattenTree(treeNode, ws.Store)
		if err != nil {
			HandleError(err)
		}

		type fileItem struct {
			Path string `json:"path"`
			Hash string `json:"hash"`
			Size int64  `json:"size"`
			Mode string `json:"mode"`
		}

		var fileList []fileItem
		for p, entry := range fileMap {
			if filesPattern != "" && filesPattern != "*" {
				matched, err := filepath.Match(filesPattern, p)
				if err != nil || !matched {
					baseMatched, berr := filepath.Match(filesPattern, filepath.Base(p))
					if berr != nil || !baseMatched {
						continue
					}
				}
			}

			fileList = append(fileList, fileItem{
				Path: p,
				Hash: entry.Hash,
				Size: entry.Size,
				Mode: entry.Mode,
			})
		}

		// Sort
		switch filesSort {
		case "size":
			sort.Slice(fileList, func(i, j int) bool {
				return fileList[i].Size > fileList[j].Size
			})
		default:
			sort.Slice(fileList, func(i, j int) bool {
				return fileList[i].Path < fileList[j].Path
			})
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"checkpoint_id": targetID,
				"files":         fileList,
				"total":         len(fileList),
			})
			return
		}

		for _, fi := range fileList {
			fmt.Printf("%-50s  %s  %s\n", fi.Path, fi.Mode, output.FormatBytes(fi.Size))
		}
	},
}

func init() {
	filesCmd.Flags().StringVar(&filesAtCP, "at", "", "Checkpoint to list files at (default HEAD)")
	filesCmd.Flags().StringVar(&filesPattern, "pattern", "", "Glob pattern to filter files")
	filesCmd.Flags().StringVar(&filesSort, "sort", "name", "Sort by: name, size")
	RootCmd.AddCommand(filesCmd)
}
