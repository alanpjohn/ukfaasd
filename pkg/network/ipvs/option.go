package ipvs

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/pkg/errors"
)

type ipvsNetworkServiceOption func(context.Context, *ipvsNetworkService) error

func WithStorage(storage api.Storage) ipvsNetworkServiceOption {
	return func(ctx context.Context, ins *ipvsNetworkService) error {
		ntwkStore, err := storage.GetNetworkStore(ctx)
		if err != nil {
			return err
		}
		ins.endpoints = ntwkStore
		return nil
	}
}

func WithSubnet(subnet string) ipvsNetworkServiceOption {
	return func(ctx context.Context, ins *ipvsNetworkService) error {
		split := strings.Split(subnet, "/")
		addr := split[0]
		mask, err := strconv.ParseInt(split[1], 10, 8)
		if err != nil {
			return errors.Wrapf(err, "could not parse mask: %s", subnet)
		}

		sub := &net.IPNet{
			IP:   net.ParseIP(addr),
			Mask: net.CIDRMask(int(mask), 32),
		}

		ins.subnet = sub
		return nil
	}
}
