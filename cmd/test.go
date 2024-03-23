package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/alanpjohn/ukfaas/pkg/machine"
	"github.com/alanpjohn/ukfaas/pkg/manager"
	"github.com/alanpjohn/ukfaas/pkg/network"
	"github.com/alanpjohn/ukfaas/pkg/store"
	"github.com/openfaas/faas-provider/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// The test command is specifically made for testing functionalities
// till a proper test suite is not implemented or to test new features
//
// TODO: Remove Test Command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Used for Testing",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		inmemory, err := store.NewStorage(ctx)
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

		manager, err := manager.InitialiseManagerV1(ctx, mService, nService, inmemory)
		if err != nil {
			return errors.Wrap(err, "Manager initialisation failed")
		}

		err = manager.Deploy(types.FunctionDeployment{
			Service: "test",
			Image:   args[0],
		})
		if err != nil {
			return err
		}

		fmt.Println("Press ENTER to scale up...")
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')

		err = manager.Scale(types.ScaleServiceRequest{
			ServiceName: "test",
			Replicas:    2,
		})
		if err != nil {
			return err
		}

		fmt.Println("Press ENTER to scale down...")
		_, _ = reader.ReadString('\n')

		err = manager.Scale(types.ScaleServiceRequest{
			ServiceName: "test",
			Replicas:    1,
		})
		if err != nil {
			return err
		}

		fmt.Println("Press ENTER to delete...")
		_, _ = reader.ReadString('\n')

		err = manager.Delete(types.DeleteFunctionRequest{
			FunctionName: "test",
		})
		if err != nil {
			return err
		}

		fmt.Println("Press ENTER to quit...")
		_, _ = reader.ReadString('\n')

		return nil
	},
}
