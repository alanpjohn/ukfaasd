package main

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/alanpjohn/ukfaas/pkg/frontend/openfaas"
	"github.com/alanpjohn/ukfaas/pkg/machine"
	"github.com/alanpjohn/ukfaas/pkg/manager"
	"github.com/alanpjohn/ukfaas/pkg/network"
	"github.com/alanpjohn/ukfaas/pkg/store"
	"github.com/alanpjohn/ukfaas/pkg/util"
)

func GetProviderCmd() *cobra.Command {
	var command = &cobra.Command{
		Use:   "start",
		Short: "Run ukfaas OpenFaaS provider daemon",
		Long: heredoc.Docf(`
		Runs the ukfaasd daemon with the OpenFaas
		provider API which connects with the OpenFaas
		gateway to give a single node unikernel 
		serverless framework
		`),
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		ctx = util.SetGitCommit(ctx, GetGitCommit())
		ctx = util.SetVersion(ctx, GetVersion())

		storage, err := store.NewStorage(ctx)
		if err != nil {
			return errors.Wrap(err, "Storage initialisation failed")
		}

		mService, err := machine.GetMachineService(ctx)
		if err != nil {
			return errors.Wrap(err, "Machine Service initialisation failed")
		}

		nService, err := network.GetNetworkService(ctx)
		if err != nil {
			return errors.Wrap(err, "Network Service initialisation failed")
		}

		manager, err := manager.InitialiseManagerV1(ctx, mService, nService, storage)
		if err != nil {
			return errors.Wrap(err, "Manager initialisation failed")
		}

		frontend, err := openfaas.GetOpenFaaSProvider(
			ctx,
			openfaas.WithManager(manager),
		)

		if err != nil {
			return errors.Wrap(err, "error initialising openfaas provider")
		}

		frontend.Serve(ctx)

		return nil
	}

	return command
}
