package cli

import (
	"fmt"
	"time"

	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var verifyFix bool

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify cryptographic integrity of all stored objects",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := GetWorkspace()
		if err != nil {
			HandleError(err)
		}

		startTime := time.Now()
		hashes, err := ws.Store.ListObjects()
		if err != nil {
			HandleError(err)
		}

		var verified, corrupted, fixed int

		for _, h := range hashes {
			_, _, err := ws.Store.ReadObject(h)
			if err != nil {
				corrupted++
				if verifyFix {
					if err := ws.Store.DeleteObject(h); err == nil {
						fixed++
					}
				}
			} else {
				verified++
			}
		}

		durationMs := time.Since(startTime).Milliseconds()

		if FlagJSON {
			output.PrintJSON(map[string]any{
				"total_objects": len(hashes),
				"verified":      verified,
				"corrupted":     corrupted,
				"fixed":         fixed,
				"duration_ms":   durationMs,
			})
			return
		}

		if corrupted == 0 {
			fmt.Printf("%s Verified %d objects with 0 errors (%dms)\n", output.Green("✔"), verified, durationMs)
		} else {
			fmt.Printf("%s Verification completed: %d valid, %s (%dms)\n",
				output.Red("✖"), verified, output.Red(fmt.Sprintf("%d corrupted", corrupted)), durationMs)
			if verifyFix {
				fmt.Printf("  Removed %d corrupted objects\n", fixed)
			}
		}
	},
}

func init() {
	verifyCmd.Flags().BoolVar(&verifyFix, "fix", false, "Remove corrupted objects")
	RootCmd.AddCommand(verifyCmd)
}
