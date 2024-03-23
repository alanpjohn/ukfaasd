package maps

import (
	"context"
	"fmt"
	"sync"

	"github.com/alanpjohn/ukfaas/pkg/api"
)

type MapStoreRepository struct{}

type mapStore struct {
	functions sync.Map
	endpoints sync.Map
}

func (m MapStoreRepository) GetFunctionStore(_ context.Context) (api.FunctionStore, error) {
	return &mapStore{
		functions: sync.Map{},
	}, nil
}

func (m MapStoreRepository) GetNetworkStore(_ context.Context) (api.NetworkStore, error) {
	return &mapStore{
		endpoints: sync.Map{},
	}, nil
}

// GetFunction implements api.FunctionStore.
func (m *mapStore) GetFunction(service string) (api.Function, error) {
	val, exists := m.functions.Load(service)
	if !exists {
		return api.Function{}, fmt.Errorf("not found")
	}
	fn, ok := val.(api.Function)
	if !ok {
		return api.Function{}, fmt.Errorf("found malformed function")
	}
	return fn, nil
}
