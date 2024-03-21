package api

import "context"

type networkState string

const (
	EndpointAdded   networkState = "EndpointAreated"
	EndpointDeleted networkState = "EndpointDeleted"
	ServiceCreated  networkState = "ServiceCreated"
	ServiceDeleted  networkState = "ServiceDeleted"
)

type NetworkEvent struct {
	Service   string
	ServiceIP string
	IP        string
	State     networkState
}

type NetworkService interface {
	Notify(context.Context, chan<- NetworkEvent) error
	NewService(context.Context, string, string) error
	DeleteService(context.Context, string) error
	AddServiceEndpoint(context.Context, string, string) error
	DeleteServiceEndpoint(context.Context, string, string) error
	Resolve(context.Context, string) (string, error)
}
