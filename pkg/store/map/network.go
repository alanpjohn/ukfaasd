package maps

import "fmt"

// GetEndpoint implements api.NetworkStore.
func (m *mapStore) GetEndpoint(service string) (string, error) {
	val, exists := m.endpoints.Load(service)
	if !exists {
		return "", fmt.Errorf("not found")
	}
	ip, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("found malformed IP")
	}
	return ip, nil
}

// PutEndpoint implements api.NetworkStore.
func (m *mapStore) PutEndpoint(service string, IP string) error {
	m.endpoints.Store(service, IP)
	return nil
}

// DeleteEndpoint implements api.NetworkStore.
func (m *mapStore) DeleteEndpoint(service string) error {
	m.endpoints.Delete(service)
	return nil
}
