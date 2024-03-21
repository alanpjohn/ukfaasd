package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/store"
	"github.com/cloudflare/ipvs"
	"github.com/cloudflare/ipvs/netmask"
	"github.com/erikh/ping"
	"github.com/pkg/errors"
	"kraftkit.sh/machine/network/iputils"
)

type ipvsNetworkService struct {
	endpoints    api.NetworkStore
	ipvsClient   ipvs.Client
	allocatedIPs sync.Map
	subnet       *net.IPNet
	notify       chan<- api.NetworkEvent
}

func GetIPVSNetworkService() (api.NetworkService, error) {
	c, err := ipvs.New()
	if err != nil {
		return &ipvsNetworkService{}, err
	}

	store := store.GetInMemoryNtwkStore()

	subnet := &net.IPNet{
		IP:   net.ParseIP("10.63.0.1"),
		Mask: net.IPv4Mask(255, 255, 0, 0),
	}

	return &ipvsNetworkService{
		ipvsClient:   c,
		endpoints:    store,
		allocatedIPs: sync.Map{},
		subnet:       subnet,
	}, nil
}

// Resolve implements api.NetworkService.
func (i *ipvsNetworkService) Resolve(_ context.Context, service string) (string, error) {
	endpoint, err := i.endpoints.GetEndpoint(service)
	log.Printf("got %s with error %v", endpoint, err)
	return endpoint, err
}

// AddServiceEndpoint implements api.NetworkService.
func (i *ipvsNetworkService) AddServiceEndpoint(_ context.Context, service string, ip string) error {
	ipvsService, err := i.retrieveIPVSService(service)
	if err != nil {
		return errors.Wrap(err, "requires existing service")
	}

	ipvsDest, err := buildDestination(ip, pkg.WatchdogPort)
	if err != nil {
		return errors.Wrapf(err, "cannot parse machine IP: %s", ip)
	}

	err = i.ipvsClient.CreateDestination(ipvsService, ipvsDest)
	if err != nil {
		return errors.Wrap(err, "cannot create IPVS Destination")
	}
	i.notify <- api.NetworkEvent{
		Service:   service,
		IP:        ip,
		ServiceIP: ipvsService.Address.String(),
		State:     api.EndpointAdded,
	}
	return nil
}

// DeleteService implements api.NetworkService.
func (i *ipvsNetworkService) DeleteService(ctx context.Context, service string) error {
	ipvsService, err := i.retrieveIPVSService(service)
	if err != nil {
		return errors.Wrap(err, "requires existing service")
	}

	err = i.ipvsClient.RemoveService(ipvsService)
	if err != nil {
		return errors.Wrap(err, "failed to delete service")
	}

	err = i.endpoints.DeleteEndpoint(service)
	if err != nil {
		return err
	}

	i.notify <- api.NetworkEvent{
		Service:   service,
		IP:        "",
		ServiceIP: ipvsService.Address.String(),
		State:     api.ServiceDeleted,
	}
	return nil

}

// DeleteServiceEndpoint implements api.NetworkService.
func (i *ipvsNetworkService) DeleteServiceEndpoint(_ context.Context, service string, ip string) error {
	ipvsService, err := i.retrieveIPVSService(service)
	if err != nil {
		return errors.Wrap(err, "requires existing service")
	}

	ipvsDest, err := buildDestination(ip, pkg.WatchdogPort)
	if err != nil {
		return errors.Wrapf(err, "cannot parse machine IP: %s", ip)
	}

	err = i.ipvsClient.RemoveDestination(ipvsService, ipvsDest)
	if err != nil {
		return errors.Wrap(err, "cannot delete IPVS Destination")
	}

	i.notify <- api.NetworkEvent{
		Service:   service,
		IP:        ip,
		ServiceIP: ipvsService.Address.String(),
		State:     api.EndpointDeleted,
	}
	return nil
}

// NewService implements api.NetworkService.
func (i *ipvsNetworkService) NewService(ctx context.Context, service string, ip string) error {
	endpoint, err := i.allocateIPWithinSubnet(ctx)
	if err != nil {
		return errors.Wrap(err, "No free service endpoits")
	}

	ipvsService, err := buildService(endpoint.String(), pkg.WatchdogPort)
	if err != nil {
		return errors.Wrapf(err, "cannot parse service IP: %s", ip)
	}

	err = i.ipvsClient.CreateService(ipvsService)
	if err != nil {
		return errors.Wrap(err, "cannot create IPVS service")
	}

	ipvsDest, err := buildDestination(ip, pkg.WatchdogPort)
	if err != nil {
		return errors.Wrapf(err, "cannot parse machine IP: %s", ip)
	}

	err = i.ipvsClient.CreateDestination(ipvsService, ipvsDest)
	if err != nil {
		return errors.Wrap(err, "cannot create IPVS Destination")
	}

	i.allocatedIPs.Store(endpoint.String(), 0)

	err = i.endpoints.PutEndpoint(service, endpoint.String())
	if err != nil {
		return errors.Wrap(err, "could not submit to store")
	}

	log.Printf("Service created at %s", ipvsService.Address)

	i.notify <- api.NetworkEvent{
		Service:   service,
		IP:        ipvsDest.Address.String(),
		ServiceIP: endpoint.String(),
		State:     api.ServiceCreated,
	}
	log.Print("Notified Manager")
	return nil
}

// Notify implements api.NetworkService.
func (i *ipvsNetworkService) Notify(_ context.Context, events chan<- api.NetworkEvent) error {
	i.notify = events
	return nil
}

func (i *ipvsNetworkService) retrieveIPVSService(service string) (ipvs.Service, error) {
	endpoint, err := i.endpoints.GetEndpoint(service)
	if err != nil {
		return ipvs.Service{}, errors.Wrapf(err, "service not found %s", service)
	}
	serviceIP := net.ParseIP(endpoint)
	return buildService(serviceIP.String(), pkg.WatchdogPort)
}

func (i *ipvsNetworkService) allocateIPWithinSubnet(ctx context.Context) (net.IP, error) {

	ip := i.subnet.IP.Mask(i.subnet.Mask)

search:
	for {
		ip = iputils.IncreaseIP(ip)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		default:
		}

		switch {
		// If the IP is not within the provided network, it is not possible to
		// increment the IP so return with an error.
		case !i.subnet.Contains(ip):
			return nil, fmt.Errorf("could not allocate IP address in %v", i.subnet.String())

		// Skip the broadcast IP address.
		case !iputils.IsUnicastIP(ip, i.subnet.Mask):
			continue

		// Skip allocated IP addresses.
		case func() bool {
			_, ok := i.allocatedIPs.Load(ip.String())
			return ok
		}():
			continue

		// Use ICMP to check if the IP is in use as a final sanity check.
		case ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond):
			continue

		default:
			break search
		}
	}

	return ip, nil
}

func buildService(ip string, port uint16) (ipvs.Service, error) {
	ipAddr, err := netip.ParseAddr(strings.Split(ip, "/")[0])
	if err != nil {
		return ipvs.Service{}, err
	}

	return ipvs.Service{
		Address:   ipAddr,
		Port:      port,
		Scheduler: "rr",
		Family:    ipvs.INET,
		FWMark:    0,
		Protocol:  ipvs.TCP,
		Flags:     ipvs.ServiceHashed,
		Netmask:   netmask.MaskFrom4([4]byte{255, 255, 255, 255}),
	}, nil
}

func buildDestination(ip string, port uint16) (ipvs.Destination, error) {
	ipAddr, err := netip.ParseAddr(strings.Split(ip, "/")[0])
	if err != nil {
		return ipvs.Destination{}, err
	}

	return ipvs.Destination{
		Address:        ipAddr,
		Port:           port,
		Family:         ipvs.INET,
		FwdMethod:      ipvs.Masquerade,
		Weight:         1,
		TunnelType:     ipvs.IPIP,
		TunnelPort:     0,
		TunnelFlags:    ipvs.TunnelEncapNoChecksum,
		UpperThreshold: 0,
		LowerThreshold: 0,
	}, nil
}
