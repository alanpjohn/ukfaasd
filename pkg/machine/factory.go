package machine

import (
	"context"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/machine/lite"
	"github.com/alanpjohn/ukfaas/pkg/store"
	"github.com/pkg/errors"
)

func GetMachineService(ctx context.Context) (api.MachineService, error) {
	containerdAddr := "/run/containerd/containerd.sock"

	storage, err := store.NewStorage(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot retrieve storage")
	}

	return lite.GetMachineServiceLite(
		ctx,
		lite.WithKraftMachineService(),
		lite.WithKraftNetworkService(),
		lite.WithPackmanager(
			containerdAddr,
			pkg.DefaultFunctionNamespace,
		),
		lite.WithStorage(storage),
	)
}
