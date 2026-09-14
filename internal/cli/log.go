package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	logLimit  int
	logAgent  string
	logTask   string
	logSince  string
	logUntil  string
	logGraph  bool
	logBranch string
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show checkpoint history DAG and changesets",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		checkpoints, err := core.ListCheckpoints(ws)
		if err != nil {
			HandleError(err)
		}

		// Apply filters
		var filtered []*core.Checkpoint
		for _, cp := range checkpoints {
			if logAgent != "" && cp.Metadata.Agent != logAgent {
				continue
			}
			if logTask != "" && !strings.Contains(strings.ToLower(cp.Metadata.Task), strings.ToLower(logTask)) {
				continue
			}
			if logSince != "" {
				sinceTime, err := parseDate(logSince)
				if err == nil && cp.CreatedAt.Before(sinceTime) {
					continue
				}
			}
			if logUntil != "" {
				untilTime, err := parseDate(logUntil)
				if err == nil && cp.CreatedAt.After(untilTime) {
					continue
				}
			}

			filtered = append(filtered, cp)
			if logLimit > 0 && len(filtered) >= logLimit {
				break
			}
		}

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"checkpoints": filtered,
			})
			return
		}

		if len(filtered) == 0 {
			fmt.Println("no checkpoints match filters")
			return
		}

		headID, _ := core.ResolveRef(ws, "HEAD")
		branchName, _ := core.ReadHead(ws)

		for _, cp := range filtered {
			marker := "* "
			headTag := ""
			if cp.ID == headID {
				headTag = fmt.Sprintf(" (%s -> %s)", output.Cyan("HEAD"), output.Green(branchName))
			}

			fmt.Printf("%s%s%s  %s",
				output.Yellow(marker),
				output.Bold(cp.ID),
				headTag,
				output.RelativeTime(cp.CreatedAt),
			)

			if cp.Metadata.Agent != "" {
				fmt.Printf("  agent:%s", output.Cyan(cp.Metadata.Agent))
			}
			if cp.Metadata.Task != "" {
				fmt.Printf("  task:%s", cp.Metadata.Task)
			}

			summary := cp.Changeset.Summary()
			fmt.Printf("  %s %s %s\n",
				output.Green(fmt.Sprintf("+%d", summary["added"])),
				output.Yellow(fmt.Sprintf("~%d", summary["modified"])),
				output.Red(fmt.Sprintf("-%d", summary["deleted"])),
			)

			if len(cp.Parents) > 1 {
				fmt.Printf("  |\\  merge from %s\n", strings.Join(cp.Parents[1:], ", "))
			}
		}
	},
}

func parseDate(s string) (time.Time, error) {
	if strings.HasSuffix(s, "h") {
		d, err := time.ParseDuration(s)
		if err == nil {
			return time.Now().Add(-d), nil
		}
	}
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		d, err := time.ParseDuration(days + "h")
		if err == nil {
			return time.Now().Add(-d * 24), nil
		}
	}
	return time.Parse(time.RFC3339, s)
}

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 20, "Maximum number of checkpoints to show")
	logCmd.Flags().StringVar(&logAgent, "agent", "", "Filter by agent identifier")
	logCmd.Flags().StringVar(&logTask, "task", "", "Filter by task description")
	logCmd.Flags().StringVar(&logSince, "since", "", "Filter checkpoints after this date or duration (e.g. 1h, 2d)")
	logCmd.Flags().StringVar(&logUntil, "until", "", "Filter checkpoints before this date")
	logCmd.Flags().BoolVar(&logGraph, "graph", false, "Show ASCII DAG visualization")
	logCmd.Flags().StringVar(&logBranch, "branch", "", "Show history for specific branch")
	RootCmd.AddCommand(logCmd)
}
