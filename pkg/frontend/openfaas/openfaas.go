package openfaas

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/pkg/errors"

	provider "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/faasd/pkg/provider/config"
)

type openFaaSProvider struct {
	handlers       types.FaaSHandlers
	config         types.FaaSConfig
	providerConfig config.ProviderConfig
}

const workingDirectoryPermission = 0644

func (o openFaaSProvider) Serve(ctx context.Context) {
	log.Printf("uk-faas provider starting..\tService Timeout: %s\n", o.config.WriteTimeout.String())
	log.Printf("Listening on: 0.0.0.0:%d\n", *o.config.TCPPort)
	provider.Serve(ctx, &o.handlers, &o.config)

}

func GetOpenFaaSProvider(ctx context.Context, opts ...any) (api.Frontend, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	writeHostsErr := os.WriteFile(path.Join(wd, "hosts"),
		[]byte(`127.0.0.1	localhost\n127.0.0.1	ukfaas.dev`), workingDirectoryPermission)

	if writeHostsErr != nil {
		return nil, fmt.Errorf("cannot write hosts file: %s", writeHostsErr)
	}

	writeResolvErr := os.WriteFile(path.Join(wd, "resolv.conf"),
		[]byte(`nameserver 8.8.8.8`), workingDirectoryPermission)

	if writeResolvErr != nil {
		return nil, fmt.Errorf("cannot write resolv.conf file: %s", writeResolvErr)
	}

	config, providerConfig, err := config.ReadFromEnv(types.OsEnv{})
	if err != nil {
		return nil, err
	}

	var provider = openFaaSProvider{
		config:         *config,
		providerConfig: *providerConfig,
	}

	for _, val := range opts {
		opt, ok := val.(OpenFaaSOption)
		if !ok {
			return nil, fmt.Errorf("invalid option given")
		}
		err := opt(ctx, &provider)
		if err != nil {
			return nil, errors.Wrap(err, "option set failed")
		}
	}

	return provider, nil
}
