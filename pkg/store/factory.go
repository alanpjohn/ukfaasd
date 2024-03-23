package store

import (
	"context"

	"github.com/alanpjohn/ukfaas/pkg/api"
	maps "github.com/alanpjohn/ukfaas/pkg/store/map"
)

var storage api.Storage = nil

func NewStorage(ctx context.Context) (api.Storage, error) {
	if storage != nil {
		return storage, nil
	}

	return maps.MapStoreRepository{}, nil
}
