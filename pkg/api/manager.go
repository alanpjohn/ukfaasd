package api

import (
	"net/url"

	"github.com/openfaas/faas-provider/types"
)

type Manager interface {
	Delete(types.DeleteFunctionRequest) error
	Deploy(types.FunctionDeployment) error
	Invoke(string) (url.URL, error)
	List() ([]types.FunctionStatus, error)
	Scale(types.ScaleServiceRequest) error
	Get(string) (types.FunctionStatus, error)
	Update(types.FunctionDeployment) error
}
