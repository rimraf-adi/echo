package cli

import (
	"fmt"
	"os"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var catAtCP string

var catCmd = &cobra.Command{
	Use:   "cat <file-path>",
	Short: "Output content of a file at current or historical checkpoint",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		filePath := args[0]
		targetRef := catAtCP
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

		entry, ok := fileMap[filePath]
		if !ok {
			HandleError(fmt.Errorf("file %s not found at checkpoint %s", filePath, targetID))
		}

		content, err := ws.Store.ReadBlob(entry.Hash)
		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"path":          filePath,
				"checkpoint_id": targetID,
				"hash":          entry.Hash,
				"size":          len(content),
				"content":       string(content),
			})
			return
		}

		_, _ = os.Stdout.Write(content)
	},
}

func init() {
	catCmd.Flags().StringVar(&catAtCP, "at", "", "Checkpoint to read file from (default HEAD)")
	RootCmd.AddCommand(catCmd)
}
