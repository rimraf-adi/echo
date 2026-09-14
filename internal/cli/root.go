package cli

import (
	"fmt"
	"os"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/output"
	"github.com/spf13/cobra"
)

var (
	FlagJSON    bool
	FlagDir     string
	FlagQuiet   bool
	FlagVerbose bool
)

var RootCmd = &cobra.Command{
	Use:   "echo",
	Short: "Echo — Fast, content-verified codebase state tracker for AI agents",
	Long: `Echo is a lightweight, Go-based codebase state tracker purpose-built for AI agent workflows.
It provides automatic checkpointing, full history traversal, Merkle tree content verification,
and deterministic rollback completely independent of git.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&FlagJSON, "json", "j", false, "Output in JSON format")
	RootCmd.PersistentFlags().StringVarP(&FlagDir, "dir", "C", "", "Run as if echo was started in <dir>")
	RootCmd.PersistentFlags().BoolVarP(&FlagQuiet, "quiet", "q", false, "Suppress non-essential output")
	RootCmd.PersistentFlags().BoolVarP(&FlagVerbose, "verbose", "v", false, "Verbose debug output")
}

// Execute runs the root CLI command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		HandleError(err)
	}
}

// GetWorkspace resolves the active workspace based on --dir or working directory
func GetWorkspace() (*core.Workspace, error) {
	dir := FlagDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	return core.Open(dir)
}

// HandleError outputs the error appropriately based on --json flag and exits with code 1
func HandleError(err error) {
	if err == nil {
		return
	}
	if FlagJSON {
		output.PrintErrorJSON(err)
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n", output.Red("error:"), err)
	}
	os.Exit(1)
}
