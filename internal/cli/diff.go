package cli

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	diffStat     bool
	diffNameOnly bool
)

var diffCmd = &cobra.Command{
	Use:   "diff [<cp-a>] [<cp-b>]",
	Short: "Show differences between checkpoints or working tree",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		var diffRes *core.DiffResult

		switch len(args) {
		case 0:
			// Diff working directory vs HEAD
			diffRes, err = core.DiffWorking(ws, "HEAD")
		case 1:
			// Diff working directory vs specified checkpoint
			diffRes, err = core.DiffWorking(ws, args[0])
		case 2:
			// Diff checkpoint A vs checkpoint B
			diffRes, err = core.DiffCheckpoints(ws, args[0], args[1])
		default:
			HandleError(fmt.Errorf("diff expects at most 2 arguments"))
		}

		if err != nil {
			HandleError(err)
		}

		if FlagJSON {
			output.PrintJSON(diffRes)
			return
		}

		if len(diffRes.Files) == 0 {
			if !FlagQuiet {
				fmt.Println("no differences")
			}
			return
		}

		for _, f := range diffRes.Files {
			if diffNameOnly {
				fmt.Println(f.Path)
				continue
			}

			if diffStat {
				fmt.Printf(" %-40s | %d %s%s\n",
					f.Path,
					f.LinesAdded+f.LinesRemoved,
					output.Green(fmt.Sprintf("+%d", f.LinesAdded)),
					output.Red(fmt.Sprintf("-%d", f.LinesRemoved)),
				)
				continue
			}

			// Full diff
			switch f.Type {
			case "added":
				fmt.Printf("%s %s (added)\n", output.Bold("diff"), f.Path)
				fmt.Printf("--- /dev/null\n+++ b/%s\n", f.Path)
			case "deleted":
				fmt.Printf("%s %s (deleted)\n", output.Bold("diff"), f.Path)
				fmt.Printf("--- a/%s\n+++ /dev/null\n", f.Path)
			case "modified":
				fmt.Printf("%s %s (modified)\n", output.Bold("diff"), f.Path)
				fmt.Printf("--- a/%s\n+++ b/%s\n", f.Path, f.Path)
			}

			for _, hunk := range f.Hunks {
				fmt.Printf("%s @@ -%d,%d +%d,%d @@%s\n",
					output.Cyan("@@"),
					hunk.OldStart, hunk.OldCount,
					hunk.NewStart, hunk.NewCount,
					output.Cyan("@@"),
				)
				for _, l := range hunk.Lines {
					switch l.Type {
					case "context":
						fmt.Printf(" %s\n", l.Content)
					case "add":
						fmt.Printf("%s\n", output.Green("+"+l.Content))
					case "delete":
						fmt.Printf("%s\n", output.Red("-"+l.Content))
					}
				}
			}
			fmt.Println()
		}
	},
}

func init() {
	diffCmd.Flags().BoolVar(&diffStat, "stat", false, "Show diffstat summary")
	diffCmd.Flags().BoolVar(&diffNameOnly, "name-only", false, "Show only names of changed files")
	RootCmd.AddCommand(diffCmd)
}
