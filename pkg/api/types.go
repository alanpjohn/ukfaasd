package api

import (
	"github.com/openfaas/faas-provider/types"
	"kraftkit.sh/unikraft/target"
)

type Function struct {
	Target     target.Target
	Deployment types.FunctionDeployment
}

func (f Function) Name() string {
	return f.Deployment.Service
}

type Result struct {
	IPs []string
}
