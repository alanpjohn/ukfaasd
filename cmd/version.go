package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Compile time variable to set version tag
	Version string
	// Compile time variable to set Git Commit Hash
	GitCommit string
)

// Version subcommand prints the version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information.",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("uk-faas version: %s\tcommit: %s\n", GetVersion(), GetGitCommit())
	},
}
