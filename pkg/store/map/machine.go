package maps

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/util"
	"k8s.io/apimachinery/pkg/types"
	"kraftkit.sh/api/machine/v1alpha1"
)

type machineStore struct {
	serviceCount sync.Map // string -> uint
	machines     sync.Map // string -> machine
	indexLock    sync.RWMutex
}

// ActiveReplicas implements api.MachineStore.
func (m *machineStore) ActiveReplicas(service string) (uint, error) {
	val, exists := m.serviceCount.Load(service)
	if exists {
		count, ok := val.(uint)
		if !ok {
			return 0, fmt.Errorf("service %s in store has broken data", service)
		}
		log.Printf("found %d active machines for service %s", count, service)
		return count, nil
	}
	return 0, fmt.Errorf("no such service %s in store", service)
}

// DeleteMachine implements api.MachineStore.
func (m *machineStore) DeleteMachine(service string, machineID types.UID) error {
	val, exists := m.serviceCount.Load(service)
	if exists {
		count := val.(uint)
		log.Printf("found %d machines for service %s", count, service)
		m.serviceCount.Store(service, count-1)
	}
	m.machines.Delete(machineID)
	return nil
}

// GetMachine implements api.MachineStore.
func (m *machineStore) GetMachine(id types.UID) (v1alpha1.Machine, error) {
	val, ok := m.machines.Load(id)
	if ok {
		machine := val.(v1alpha1.Machine)
		return machine, nil
	}
	return v1alpha1.Machine{}, fmt.Errorf("no such machine %s in store", id)
}

// ListMachines implements api.MachineStore.
func (m *machineStore) ListMachines(service string) ([]v1alpha1.Machine, error) {
	machines := []v1alpha1.Machine{}
	m.indexLock.RLock()
	defer m.indexLock.RUnlock()

	m.machines.Range(func(key, value any) bool {
		machine := value.(v1alpha1.Machine)
		if machine.Annotations["ukfaas.io/service"] == service {
			machines = append(machines, machine)
		}
		return true
	})
	return machines, nil
}

// PopMachine implements api.MachineStore.
func (m *machineStore) PopMachine(service string) (v1alpha1.Machine, error) {
	selected := v1alpha1.Machine{}
	m.indexLock.RLock()
	defer m.indexLock.RUnlock()

	m.machines.Range(func(key, value any) bool {
		machine := value.(v1alpha1.Machine)
		if machine.Labels["ukfaas.io/service"] == service && util.IsActive(machine.Status.State) {
			selected = machine
			return false
		}
		return true
	})
	return selected, nil
}

// PutMachine implements api.MachineStore.
func (m *machineStore) PutMachine(service string, machine v1alpha1.Machine) error {
	var (
		exists    bool
		wasActive bool = false
		notActive bool
	)

	oldMachine, err := m.GetMachine(machine.GetUID())
	if err == nil {
		wasActive = util.IsActive(oldMachine.Status.State)
	}
	notActive = !util.IsActive(machine.Status.State)

	val, exists := m.serviceCount.Load(service)
	if exists {
		count := val.(uint)
		if notActive && wasActive {
			m.serviceCount.Store(service, count-1)
		} else if !notActive {
			m.serviceCount.Store(service, count+1)
		}
	} else {
		m.serviceCount.Store(service, uint(1))
	}

	m.machines.Store(machine.UID, machine)
	return nil
}

func (m MapStoreRepository) GetMachineStore(_ context.Context) (api.MachineStore, error) {
	return &machineStore{
		machines:     sync.Map{},
		serviceCount: sync.Map{},
		indexLock:    sync.RWMutex{},
	}, nil
}
