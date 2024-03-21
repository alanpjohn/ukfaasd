package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/MakeNowJust/heredoc"
	provider "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/logs"
	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"
	faasdlogs "github.com/openfaas/faasd/pkg/logs"
	"github.com/openfaas/faasd/pkg/provider/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/alanpjohn/ukfaas/pkg/handlers"
	"github.com/alanpjohn/ukfaas/pkg/machine"
	"github.com/alanpjohn/ukfaas/pkg/manager"
	"github.com/alanpjohn/ukfaas/pkg/network"
)

const workingDirectoryPermission = 0644

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
		config, providerConfig, err := config.ReadFromEnv(types.OsEnv{})
		if err != nil {
			return err
		}

		log.Printf("uk-faas provider starting..\tService Timeout: %s\n", config.WriteTimeout.String())

		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		writeHostsErr := os.WriteFile(path.Join(wd, "hosts"),
			[]byte(`127.0.0.1	localhost\n127.0.0.1	ukfaas.dev`), workingDirectoryPermission)

		if writeHostsErr != nil {
			return fmt.Errorf("cannot write hosts file: %s", writeHostsErr)
		}

		writeResolvErr := os.WriteFile(path.Join(wd, "resolv.conf"),
			[]byte(`nameserver 8.8.8.8`), workingDirectoryPermission)

		if writeResolvErr != nil {
			return fmt.Errorf("cannot write resolv.conf file: %s", writeResolvErr)
		}

		// baseUserSecretsPath := path.Join(wd, "secrets")

		ctx := context.Background()

		containerdAddr := providerConfig.Sock

		mService, err := machine.GetMachineServiceBeta(ctx, containerdAddr, pkg.DefaultContainerdNamespace)
		if err != nil {
			return errors.Wrap(err, "Machine Service initialisation failed")
		}

		nService, err := network.GetIPVSNetworkService()
		if err != nil {
			return errors.Wrap(err, "Network Service initialisation failed")
		}

		manager := manager.InitialiseManagerV1(ctx, mService, nService)

		invokeResolver := handlers.NewInvokeResolver(manager)

		bootstrapHandlers := types.FaaSHandlers{
			DeployFunction: handlers.MakeDeployHandler(manager),
			DeleteFunction: handlers.MakeDeleteHandler(manager),
			FunctionLister: handlers.MakeListHandler(manager),
			FunctionStatus: handlers.MakeFunctionStatusHandler(manager),
			ScaleFunction:  handlers.MakeScaleHandler(manager),
			UpdateFunction: handlers.MakeUpdateHandler(manager),
			FunctionProxy:  proxy.NewHandlerFunc(*config, invokeResolver, true),

			Health:          func(w http.ResponseWriter, r *http.Request) {},
			Info:            handlers.MakeInfoHandler(GetVersion(), GetGitCommit()),
			ListNamespaces:  handlers.MakeNamespaceListerHandler(),
			MutateNamespace: handlers.MakeNamespaceMutateHandler(),
			Secrets:         handlers.MakeSecretHandler(),
			Logs:            logs.NewLogHandlerFunc(faasdlogs.New(), config.ReadTimeout),
		}

		log.Printf("Listening on: 0.0.0.0:%d\n", *config.TCPPort)
		provider.Serve(ctx, &bootstrapHandlers, config)
		return nil
	}

	return command
}
