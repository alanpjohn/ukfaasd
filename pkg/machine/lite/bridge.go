package lite

import (
	"context"
	"net"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
)

func initNetworkBridge(ctx context.Context, controller networkapi.NetworkService) error {
	addr, err := netlink.ParseAddr(pkg.BridgeSubnet)
	if err != nil {
		return errors.Wrapf(err, "could not parse subnet %s", pkg.BridgeSubnet)
	}

	_, err = controller.Get(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openfaas0",
		},
	})

	if err == nil {
		return nil
	}

	_, err = controller.Create(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openfaas0",
		},
		Spec: networkapi.NetworkSpec{
			Gateway: addr.IP.String(),
			Netmask: net.IP(addr.Mask).String(),
		},
	})

	return err
}
