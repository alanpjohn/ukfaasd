package openfaas

import (
	"context"
	"net/http"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/frontend/openfaas/handlers"
	"github.com/alanpjohn/ukfaas/pkg/util"
	"github.com/openfaas/faas-provider/logs"
	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"
	faasdlogs "github.com/openfaas/faasd/pkg/logs"
)

type OpenFaaSOption func(context.Context, *openFaaSProvider) error

func WithManager(manager api.Manager) OpenFaaSOption {
	return func(ctx context.Context, oc *openFaaSProvider) error {
		invokeResolver := handlers.NewInvokeResolver(manager)

		bootstrapHandlers := types.FaaSHandlers{
			DeployFunction: handlers.MakeDeployHandler(manager),
			DeleteFunction: handlers.MakeDeleteHandler(manager),
			FunctionLister: handlers.MakeListHandler(manager),
			FunctionStatus: handlers.MakeFunctionStatusHandler(manager),
			ScaleFunction:  handlers.MakeScaleHandler(manager),
			UpdateFunction: handlers.MakeUpdateHandler(manager),
			FunctionProxy:  proxy.NewHandlerFunc(oc.config, invokeResolver, true),

			Health:          func(w http.ResponseWriter, r *http.Request) {},
			Info:            handlers.MakeInfoHandler(util.GetVersion(ctx), util.GetGitCommit(ctx)),
			ListNamespaces:  handlers.MakeNamespaceListerHandler(),
			MutateNamespace: handlers.MakeNamespaceMutateHandler(),
			Secrets:         handlers.MakeSecretHandler(),
			Logs:            logs.NewLogHandlerFunc(faasdlogs.New(), oc.config.ReadTimeout),
		}

		oc.handlers = bootstrapHandlers
		return nil
	}
}
