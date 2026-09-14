package cli

import (
	"fmt"
	"path/filepath"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	searchAtCP    string
	searchLimit   int
	searchContext int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across tracked files",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		query := args[0]
		targetRef := searchAtCP
		if targetRef == "" {
			targetRef = "HEAD"
		}

		targetID, err := core.ResolveRef(ws, targetRef)
		if err != nil {
			HandleError(err)
		}

		dbPath := filepath.Join(ws.EchoDir, "index.db")
		idx, err := index.OpenIndex(dbPath)
		if err != nil {
			HandleError(err)
		}
		defer idx.Close()

		results, err := idx.Search(targetID, query, searchLimit, searchContext)
		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"query":         query,
				"checkpoint_id": targetID,
				"matches":       results,
				"total_matches": len(results),
			})
			return
		}

		if len(results) == 0 {
			if !FlagQuiet {
				fmt.Printf("no matches for %q\n", query)
			}
			return
		}

		for _, r := range results {
			fmt.Printf("%s:%d\n", output.Bold(r.Path), r.LineNumber)
			for _, b := range r.ContextBefore {
				fmt.Printf("  %s\n", b)
			}
			fmt.Printf("  %s\n", output.Green(r.Content))
			for _, a := range r.ContextAfter {
				fmt.Printf("  %s\n", a)
			}
			fmt.Println()
		}
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchAtCP, "at", "", "Checkpoint to search (default HEAD)")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 50, "Maximum number of matches")
	searchCmd.Flags().IntVarP(&searchContext, "context", "c", 2, "Lines of context before/after matches")
	RootCmd.AddCommand(searchCmd)
}
