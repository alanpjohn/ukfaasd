package handlers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/alanpjohn/ukfaas/pkg/api"
)

type InvokeResolver struct {
	manager api.Manager
}

func NewInvokeResolver(manager api.Manager) *InvokeResolver {
	return &InvokeResolver{
		manager,
	}
}

func (i *InvokeResolver) Resolve(functionName string) (url.URL, error) {
	actualFunctionName := functionName
	if strings.Contains(functionName, ".") {
		actualFunctionName = strings.TrimSuffix(functionName, "."+pkg.DefaultFunctionNamespace)
	}
	endpoint, err := i.manager.Invoke(actualFunctionName)
	if err != nil {
		return url.URL{}, fmt.Errorf("%s not found", actualFunctionName)
	}
	return endpoint, nil
}
