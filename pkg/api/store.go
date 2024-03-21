package api

import (
	"k8s.io/apimachinery/pkg/types"
	"kraftkit.sh/api/machine/v1alpha1"
)

type FunctionStore interface {
	ListFunctions() ([]Function, error)
	PutFunction(string, Function) error
	GetFunction(string) (Function, error)
	DeleteFunction(string) error
}

type NetworkStore interface {
	DeleteEndpoint(string) error
	PutEndpoint(string, string) error
	GetEndpoint(string) (string, error)
}

type MachineStore interface {
	PutMachine(string, v1alpha1.Machine) error
	GetMachine(types.UID) (v1alpha1.Machine, error)
	ListMachines(string) ([]v1alpha1.Machine, error)
	DeleteMachine(string, types.UID) error
	PopMachine(string) (v1alpha1.Machine, error)
	ActiveReplicas(string) (uint, error)
}
