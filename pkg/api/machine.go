package api

import (
	"context"

	"kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/unikraft/target"
)

type MachineEvent struct {
	Service string
	IP      string
	Status  v1alpha1.MachineStatus
}

type MachineService interface {
	Notify(context.Context, chan<- MachineEvent) error
	PullImage(context.Context, string) (target.Target, error)
	Deploy(context.Context, Function) error
	Scale(context.Context, Function, uint) error
	Delete(context.Context, Function) error
	Replicas(context.Context, string) (uint, error)
}
