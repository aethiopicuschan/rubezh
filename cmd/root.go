package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "rubezh",
	Short: "Enforce external test packages in Go",
	Long: `Rubezh is a Go linter that requires test files to use an external test
package whose name ends in _test (for example, package foo_test).

This keeps tests focused on the package's public API instead of its unexported
implementation.`,
	RunE:          run,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func run(cmd *cobra.Command, args []string) (err error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return
	}
	violations, err := lint(cmd.ErrOrStderr(), args, cfg)
	if err != nil {
		return
	}
	if violations > 0 {
		err = fmt.Errorf("found %d test file(s) using an internal package", violations)
		return
	}
	return
}

func Execute() (err error) {
	err = rootCmd.Execute()
	return
}

func init() {
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "path to a JSON or YAML configuration file")

	bi, ok := debug.ReadBuildInfo()
	if ok && bi.Main.Version != "" {
		rootCmd.Version = bi.Main.Version
	} else {
		rootCmd.Version = "unknown"
	}
}
