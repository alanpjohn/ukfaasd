package network

import (
	"context"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/network/ipvs"
	"github.com/alanpjohn/ukfaas/pkg/store"
	"github.com/pkg/errors"
)

func GetNetworkService(ctx context.Context) (api.NetworkService, error) {
	subnet := "10.63.0.1/16"

	storage, err := store.NewStorage(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot retrieve storage")
	}

	return ipvs.GetIPVSNetworkService(
		ctx,
		ipvs.WithStorage(storage),
		ipvs.WithSubnet(subnet),
	)
}
