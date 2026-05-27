package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time by goreleaser via:
//
//	-ldflags "-X github.com/G-MAKROGLOU/migr8/cmd.version={{.Version}}"
//
// Local / untagged builds keep the sentinel value "dev".
var version = "dev"

var (
	infraConfigPath string
	infraConfig     = InfraConfig{}

	rootCmd = &cobra.Command{Use: "migr8", Version: version}
)

// Execute starts the root cmd
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
