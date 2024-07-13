package cmd

import (
	"fmt"
	"os"

	"github.com/G-MAKROGLOU/infrastructure"
	"github.com/spf13/cobra"
)

var (
	infraConfigPath string
	infraConfig     = infrastructure.InfraConfig{}

	rootCmd = &cobra.Command{Use: "migr8"}
)

// Execute starts the root cmd
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
