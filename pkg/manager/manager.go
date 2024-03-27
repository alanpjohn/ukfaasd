package manager

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/alanpjohn/ukfaas/pkg/util"
	faas "github.com/openfaas/faas-provider/types"
	"github.com/pkg/errors"
)

type managerV1 struct {
	mService api.MachineService
	nService api.NetworkService
	fstore   api.FunctionStore
}

func InitialiseManagerV1(ctx context.Context, m api.MachineService, n api.NetworkService, s api.Storage) (api.Manager, error) {
	f, err := s.GetFunctionStore(ctx)
	if err != nil {
		return nil, err
	}
	newManager := &managerV1{
		mService: m,
		nService: n,
		fstore:   f,
	}

	go newManager.listenMachineEvents(ctx)
	go newManager.listenNetworkEvents(ctx)

	return newManager, nil
}

func (manager *managerV1) listenMachineEvents(ctx context.Context) {
	mEvents := make(chan api.MachineEvent)
	manager.mService.Notify(ctx, mEvents)
	log.Printf("listening for machine events")
loop:
	for {
		select {
		case mEvent := <-mEvents:
			log.Printf("Machine Event Recieved:\t%s\t%s\t%s\n", mEvent.Service, mEvent.IP, mEvent.Status.State)
			service := mEvent.Service
			var (
				resolveErr error
				serviceErr error
			)
			_, resolveErr = manager.nService.Resolve(ctx, service)
			if util.IsActive(mEvent.Status.State) {
				if resolveErr == nil {
					serviceErr = manager.nService.AddServiceEndpoint(ctx, mEvent.Service, mEvent.IP)
				} else {
					serviceErr = manager.nService.NewService(ctx, mEvent.Service, mEvent.IP)
				}
			} else if resolveErr == nil {
				serviceErr = manager.nService.DeleteServiceEndpoint(ctx, mEvent.Service, mEvent.IP)
			}
			if serviceErr != nil {
				log.Printf("[ERROR] %v", serviceErr)
			}
			log.Printf("Machine Event Processed:\t%s\t%s\t%s\n", mEvent.Service, mEvent.IP, mEvent.Status.State)
		case <-ctx.Done():
			log.Printf("Shut down")
			break loop
		}
	}
}

func (manager *managerV1) listenNetworkEvents(ctx context.Context) {
	nEvents := make(chan api.NetworkEvent)
	manager.nService.Notify(ctx, nEvents)
	log.Printf("listening for network events")

loop:
	for {
		select {
		case nEvent := <-nEvents:
			log.Printf("Network Event %s: service=%s service_ip=%s ip=%s",
				nEvent.State,
				nEvent.Service,
				nEvent.ServiceIP,
				nEvent.IP,
			)
		case <-ctx.Done():
			log.Printf("Shut down")
			break loop
		}
	}
	log.Fatal("Loop broken")

}

func (manager *managerV1) Delete(req faas.DeleteFunctionRequest) error {
	service := req.FunctionName
	fn, err := manager.fstore.GetFunction(service)
	if err != nil {
		return fmt.Errorf("%s does not exist", service)
	}

	err = manager.mService.Delete(context.TODO(), fn)
	if err != nil {
		return errors.Wrapf(err, "error deleting machines")
	}

	err = manager.nService.DeleteService(context.TODO(), service)
	if err != nil {
		return errors.Wrapf(err, "error deleting network service")
	}

	err = manager.fstore.DeleteFunction(service)

	return err
}

func (manager *managerV1) Deploy(req faas.FunctionDeployment) error {
	service := req.Service
	_, err := manager.Get(service)
	if err == nil {
		return fmt.Errorf("%s already exists", service)
	}

	ctx := context.Background()
	targ, err := manager.mService.PullImage(ctx, req.Image)
	if err != nil {
		return errors.Wrap(err, "Pull image failed")
	}

	fn := api.Function{
		Target:     targ,
		Deployment: req,
	}

	err = manager.fstore.PutFunction(service, fn)
	if err != nil {
		return errors.Wrap(err, "Storing target details")
	}

	log.Printf("function regsitration for %s successful", fn.Deployment.Service)

	err = manager.mService.Deploy(context.TODO(), fn)
	if err != nil {
		return errors.Wrap(err, "Deployment failed")
	}
	return nil
}

func (manager *managerV1) Invoke(service string) (url.URL, error) {
	ctx := context.Background()
	ip, err := manager.nService.Resolve(ctx, service)
	if err != nil {
		return url.URL{}, err
	}
	ipaddr := strings.Split(ip, ":")[0]
	port := strings.Split(ip, ":")[1]
	endpoint, err := url.Parse(fmt.Sprintf("http://%s:%s", ipaddr, port))
	if err != nil {
		log.Printf("[Invoke] Error parsing IP: %s", err)
		return url.URL{}, err
	}

	if replicas, err := manager.mService.Replicas(ctx, service); err == nil && replicas == 0 {
		fn, err := manager.fstore.GetFunction(service)
		if err != nil {
			return url.URL{}, errors.Wrapf(err, "function details not found: %s", service)
		}
		manager.mService.Scale(ctx, fn, 1)

		_, err = http.Get(endpoint.String())
		for err != nil {
			_, err = http.Get(endpoint.String())
		}
	}

	return *endpoint, nil
}

func (manager *managerV1) List() ([]faas.FunctionStatus, error) {
	res := []faas.FunctionStatus{}
	fns, err := manager.fstore.ListFunctions()
	if err != nil {
		return res, errors.Wrap(err, "cannot retrieve functions")
	}

	ctx := context.Background()
	for _, fn := range fns {
		var replicas uint
		if replicas, err = manager.mService.Replicas(ctx, fn.Name()); err != nil {
			replicas = 0
		}

		res = append(res, faas.FunctionStatus{
			Name:                   fn.Name(),
			Image:                  fn.Deployment.Service,
			Namespace:              fn.Deployment.Image,
			EnvProcess:             fn.Deployment.EnvProcess,
			EnvVars:                fn.Deployment.EnvVars,
			Constraints:            fn.Deployment.Constraints,
			Secrets:                fn.Deployment.Secrets,
			Labels:                 fn.Deployment.Labels,
			Annotations:            fn.Deployment.Annotations,
			ReadOnlyRootFilesystem: fn.Deployment.ReadOnlyRootFilesystem,
			Limits:                 fn.Deployment.Limits,
			Requests:               fn.Deployment.Requests,
			AvailableReplicas:      uint64(replicas),
			Replicas:               uint64(replicas),
		})
	}

	return res, nil
}

func (manager *managerV1) Scale(req faas.ScaleServiceRequest) error {
	service := req.ServiceName
	replicas := req.Replicas

	fn, err := manager.fstore.GetFunction(service)
	if err != nil {
		return errors.Wrap(err, "cannot retrieve function from store")
	}

	return manager.mService.Scale(context.TODO(), fn, uint(replicas))
}

func (manager *managerV1) Get(service string) (faas.FunctionStatus, error) {
	fn, err := manager.fstore.GetFunction(service)
	if err != nil {
		return faas.FunctionStatus{}, errors.Wrap(err, "cannot retrieve function from store")
	}
	res := faas.FunctionStatus{
		Name:                   fn.Name(),
		Image:                  fn.Deployment.Service,
		Namespace:              fn.Deployment.Image,
		EnvProcess:             fn.Deployment.EnvProcess,
		EnvVars:                fn.Deployment.EnvVars,
		Constraints:            fn.Deployment.Constraints,
		Secrets:                fn.Deployment.Secrets,
		Labels:                 fn.Deployment.Labels,
		Annotations:            fn.Deployment.Annotations,
		ReadOnlyRootFilesystem: fn.Deployment.ReadOnlyRootFilesystem,
		Limits:                 fn.Deployment.Limits,
		Requests:               fn.Deployment.Requests,
	}
	return res, nil
}

func (manager *managerV1) Update(req faas.FunctionDeployment) error {
	service := req.Service
	oldfn, err := manager.fstore.GetFunction(service)
	if err != nil {
		return errors.Wrap(err, "cannot retrieve function from store")
	}

	var (
		imageChanged      bool = false
		parametersChanged bool = false
	)

	if oldfn.Deployment.Image != req.Image {
		imageChanged = true
	}

	if oldfn.Deployment.EnvProcess != req.EnvProcess ||
		oldfn.Deployment.Limits.CPU != req.Limits.CPU ||
		oldfn.Deployment.Limits.Memory != req.Limits.Memory ||
		oldfn.Deployment.Requests.CPU != req.Requests.CPU ||
		oldfn.Deployment.Requests.Memory != req.Requests.Memory {
		parametersChanged = true
	}

	updatedfn := oldfn
	oldfn.Deployment = req

	ctx := context.Background()
	if imageChanged {

		targ, err := manager.mService.PullImage(ctx, req.Image)
		if err != nil {
			return errors.Wrap(err, "Pull image failed")
		}

		updatedfn.Target = targ

		err = manager.fstore.PutFunction(service, oldfn)
		if err != nil {
			return errors.Wrap(err, "Storing target details")
		}
	}

	manager.fstore.PutFunction(service, updatedfn)

	if imageChanged || parametersChanged {
		manager.mService.Delete(ctx, oldfn)
		manager.mService.Deploy(ctx, updatedfn)
	}

	return nil
}
