package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	infraConfigPath string
	infraConfig     = InfraConfig{}

	rootCmd = &cobra.Command{Use: "migr8", Version: "1.0.0"}
)

// Execute starts the root cmd
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
