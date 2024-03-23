package maps

import "github.com/alanpjohn/ukfaas/pkg/api"

// PutFunction implements api.FunctionStore.
func (m *mapStore) PutFunction(service string, fn api.Function) error {
	m.functions.Store(service, fn)
	return nil
}

// ListFunctions implements api.FunctionStore.
func (m *mapStore) ListFunctions() ([]api.Function, error) {
	var funcs []api.Function = []api.Function{}
	m.functions.Range(func(key, value any) bool {
		fn, ok := value.(api.Function)
		if !ok {
			return true
		}
		funcs = append(funcs, fn)
		return true
	})
	return funcs, nil
}

// DeleteFunction implements api.FunctionStore.
func (m *mapStore) DeleteFunction(service string) error {
	m.functions.Delete(service)
	return nil
}
