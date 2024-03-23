package main

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// GetRootCmd generates the Root Command
// for ukfaas
//
// TODO: Refer kraftkit.sh/internal/cli/kraft to see how it manages subcommands and preRuns
func GetRootCmd() *cobra.Command {
	var command = &cobra.Command{
		Use:   "ukfaas SUBCOMMAND [FLAGS]",
		Short: "Run ukfaas OpenFaaS provider daemon",
		Long: heredoc.Docf(`
			Run OpenFaas with the power of unikernels
			with ukfaas, a OpenFaas Provider that can
			run unikernels as functions for serverless
			computing

			Version: %s
			GitCommit: %s
			Github: https://github.com/alanpjohn/ukfaas
			
			More about unikraft at https://unikraft.org
		`, GetVersion(), GetGitCommit()),
	}

	command.AddCommand(versionCmd)
	command.AddCommand(upCmd)

	command.AddCommand(GetProviderCmd())
	command.AddCommand(testCmd) // TODO: Remove test command

	command.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.Help()

		return nil
	}

	return command
}

// GetVersion gets the latest version
// from the Version variable or sets
// it to "dirty" if not set
func GetVersion() string {
	if len(Version) == 0 {
		return "dirty"
	}
	return Version
}

// GetCommit gets the latest Git commit hash
// from the GitCommit variable or sets
// it to "dev" if not set
func GetGitCommit() string {
	if len(GitCommit) == 0 {
		return "dev"
	}
	return GitCommit
}
