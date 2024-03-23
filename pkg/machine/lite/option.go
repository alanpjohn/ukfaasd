package lite

import (
	"context"
	"fmt"
	"path/filepath"

	zip "api.zip"
	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/pkg/errors"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/machine/network/bridge"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/oci"
	"kraftkit.sh/packmanager"
	kraftStore "kraftkit.sh/store"
)

type MachineServiceLiteOption func(context.Context, *machineServiceLite) error

func WithPackmanager(containerdAddr string, namespace string) MachineServiceLiteOption {
	return func(ctx context.Context, msb *machineServiceLite) error {
		err := packmanager.InitUmbrellaManager(ctx, []func(*packmanager.UmbrellaManager) error{
			RegisterPackageManager(ctx, containerdAddr, namespace),
		})
		if err != nil {
			return errors.Wrap(err, "pack manager register failed")
		}
		pms, err := packmanager.PackageManagers()
		if err != nil {
			return errors.Wrap(err, "pack managers retrieval failed")
		}
		pm, exists := pms[oci.OCIFormat]

		if !exists {
			return fmt.Errorf("cannot retirve oci pack manager")
		}

		msb.pm = pm
		return nil
	}
}

func WithKraftMachineService() MachineServiceLiteOption {
	return func(ctx context.Context, msb *machineServiceLite) error {
		supportedPlatforms := []mplatform.Platform{mplatform.PlatformFirecracker, mplatform.PlatformKVM}

		machineControllers := make(map[mplatform.Platform]machineapi.MachineService)
		for _, platform := range supportedPlatforms {
			machineStrategy, ok := mplatform.Strategies()[platform]
			if !ok {
				return fmt.Errorf("platform %s not supported", platform)
			}
			machineController, err := machineStrategy.NewMachineV1alpha1(ctx)
			if err != nil {
				return err
			}
			machineControllers[platform] = machineController
		}
		msb.machineControllers = machineControllers
		return nil
	}
}

func WithKraftNetworkService() MachineServiceLiteOption {
	return func(ctx context.Context, msb *machineServiceLite) error {
		service, err := bridge.NewNetworkServiceV1alpha1(ctx)
		if err != nil {
			return err
		}

		embeddedStore, err := kraftStore.NewEmbeddedStore[networkapi.NetworkSpec, networkapi.NetworkStatus](
			filepath.Join(
				config.G[config.KraftKit](ctx).RuntimeDir,
				"networkv1alpha1",
			),
		)
		if err != nil {
			return err
		}

		networkService, err := networkapi.NewNetworkServiceHandler(
			ctx,
			service,
			zip.WithStore[networkapi.NetworkSpec, networkapi.NetworkStatus](embeddedStore, zip.StoreRehydrationSpecNil),
		)
		if err != nil {
			return err
		}
		msb.networks = networkService
		return nil
	}
}

func WithStorage(storage api.Storage) MachineServiceLiteOption {
	return func(ctx context.Context, msb *machineServiceLite) error {
		machineStore, err := storage.GetMachineStore(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get storage")
		}
		msb.mStore = machineStore
		return nil
	}
}
