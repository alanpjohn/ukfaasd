package lite

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/alanpjohn/ukfaas/pkg"
	"github.com/alanpjohn/ukfaas/pkg/api"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/initrd"
	machinename "kraftkit.sh/machine/name"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/oci"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

func RegisterPackageManager(ctx context.Context, addr string, namespace string) func(u *packmanager.UmbrellaManager) error {
	return func(u *packmanager.UmbrellaManager) error {
		return u.RegisterPackageManager(
			oci.OCIFormat,
			oci.NewOCIManager,
			oci.WithDefaultAuth(),
			oci.WithDefaultRegistries(),
			oci.WithContainerd(ctx, addr, namespace),
		)
	}
}

type machineServiceLite struct {
	notify chan<- api.MachineEvent
	// taskQueue chan MachineTask
	mStore             api.MachineStore
	pm                 packmanager.PackageManager
	networks           networkapi.NetworkService
	machineControllers map[mplatform.Platform]machineapi.MachineService
	// lock      sync.Mutex
}

func GetMachineServiceLite(ctx context.Context, opts ...any) (api.MachineService, error) {
	ms := &machineServiceLite{}

	for _, val := range opts {
		opt, ok := val.(MachineServiceLiteOption)
		if !ok {
			return ms, fmt.Errorf("invalid option provided")
		}

		err := opt(ctx, ms)
		if err != nil {
			return ms, errors.Wrap(err, "error applying opt")
		}
	}

	if ms.networks != nil {
		err := initNetworkBridge(ctx, ms.networks)
		if err != nil {
			return nil, errors.Wrap(err, "error setting up openfaas bridge network")
		}
	} else {
		return nil, fmt.Errorf("error: no network service created")
	}

	return ms, nil
}

func (ms *machineServiceLite) PullImage(ctx context.Context, imageRef string) (target.Target, error) {
	log.Printf("Looking for %s", imageRef)
	qopts := []packmanager.QueryOption{
		packmanager.WithTypes(unikraft.ComponentTypeApp),
		packmanager.WithName(imageRef),
	}

	log.Printf("Starting to catalog local")
	packs, err := ms.pm.Catalog(ctx, qopts...)
	if err != nil {
		return nil, errors.Wrap(err, "could not query local catalog")
	}
	if len(packs) == 0 {
		log.Printf("Starting to catalog remotely")
		qopts = append(qopts, packmanager.WithRemote(true))
		packs, err = ms.pm.Catalog(ctx, qopts...)
		if err != nil {
			return nil, errors.Wrap(err, "could not query remote catalog")
		}
	}

	var selectedpackage pack.Package

	if len(packs) == 0 {
		return nil, fmt.Errorf("no images found")
	} else if len(packs) == 1 {
		selectedpackage = packs[0]
	} else {
		return nil, fmt.Errorf("found multiple packages")
	}

	if exists, _, err := selectedpackage.PulledAt(ctx); !exists || err != nil {
		selectedpackage.Pull(ctx)
	}

	uid := uuid.NewUUID()
	stateDir := filepath.Join(pkg.OCIDirectory, string(uid))
	if err := os.MkdirAll(stateDir, fs.ModeSetgid|0o775); err != nil {
		return nil, err
	}

	// Clean up the package directory if an error occurs before returning.
	defer func() {
		if err != nil {
			os.RemoveAll(stateDir)
		}
	}()

	if err := selectedpackage.Unpack(
		ctx,
		stateDir,
	); err != nil {
		return nil, fmt.Errorf("unpacking the image: %w", err)
	}

	targ, ok := selectedpackage.(target.Target)
	if !ok {
		return nil, fmt.Errorf("package does not convert to target")
	}

	return targ, nil
}

// Notify implements api.MachineService.
func (ms *machineServiceLite) Notify(ctx context.Context, events chan<- api.MachineEvent) error {
	ms.notify = events
	return nil
}

// Deploy implements api.MachineService.
func (ms *machineServiceLite) Deploy(ctx context.Context, fn api.Function) error {
	// m.notify <- fn.Deployment.Service
	log.Printf("machine deployment request for %s", fn.Deployment.Service)
	m, err := ms.machineFromFunction(ctx, fn)
	if err != nil {
		return err
	}

	machine, err := ms.create(ctx, &m)
	if err != nil {
		return err
	}
	ms.notify <- api.MachineEvent{
		Service: fn.Deployment.Service,
		Status:  machine.Status,
		IP:      machine.Spec.Networks[0].Interfaces[0].Spec.CIDR,
	}

	// ms.taskQueue <- MachineTask{
	// 	MachineTaskType: MachineCreation,
	// 	Machine:         &m,
	// }
	// log.Printf("machine creation task scheduled for %s", m.GetUID())
	return ms.mStore.PutMachine(fn.Deployment.Service, *machine)
}

// Scale implements api.MachineService.
func (ms *machineServiceLite) Scale(ctx context.Context, fn api.Function, replicas uint) error {
	service := fn.Deployment.Service
	got, err := ms.mStore.ActiveReplicas(service)
	if err != nil {
		return err
	}
	if got == replicas {
		return nil
	}
	if got == 0 && replicas > 0 {
		m, err := ms.machineFromFunction(ctx, fn)
		if err != nil {
			return err
		}

		machine, err := ms.create(ctx, &m)
		if err != nil {
			return err
		}

		ms.mStore.PutMachine(service, *machine)
		ms.notify <- api.MachineEvent{
			Service: fn.Deployment.Service,
			Status:  machine.Status,
			IP:      machine.Spec.Networks[0].Interfaces[0].Spec.CIDR,
		}
		// ms.taskQueue <- MachineTask{
		// 	MachineTaskType: MachineCreation,
		// 	Machine:         &machine,
		// }
		// log.Printf("machine creation task scheduled for %s", machine.GetUID())
		got += 1
	}
	if got < replicas {
		for i := got; i < replicas; i++ {
			newMachine, err := ms.machineFromFunction(ctx, fn)
			if err != nil {
				return err
			}
			machine, err := ms.create(ctx, &newMachine)
			if err != nil {
				return err
			}
			ms.mStore.PutMachine(service, *machine)
			ms.notify <- api.MachineEvent{
				Service: fn.Deployment.Service,
				Status:  machine.Status,
				IP:      machine.Spec.Networks[0].Interfaces[0].Spec.CIDR,
			}
			// ms.taskQueue <- MachineTask{
			// 	Machine:         &newMachine,
			// 	MachineTaskType: MachineCreation,
			// }
			// log.Printf("machine creation task scheduled for %s", machine.GetUID())
		}
	} else {
		for i := got; i > replicas; i-- {
			m, err := ms.mStore.PopMachine(service)
			if err != nil {
				return err
			}
			machine, err := ms.destroy(ctx, &m)
			if err != nil {
				return err
			}
			ms.notify <- api.MachineEvent{
				Service: fn.Deployment.Service,
				Status:  machine.Status,
				IP:      machine.Spec.Networks[0].Interfaces[0].Spec.CIDR,
			}
			ms.mStore.DeleteMachine(service, machine.GetUID())
			// machine.Status.State = machineapi.MachineStateSuspended
			// ms.mStore.PutMachine(string(machine.UID), machine)
			// ms.taskQueue <- MachineTask{
			// 	Machine:         &machine,
			// 	MachineTaskType: MachineDeletion,
			// }
			// log.Printf("machine deletion task scheduled for %s", machine.GetUID())
		}
	}
	return nil
}

// Delete implements api.MachineService.
func (ms *machineServiceLite) Delete(ctx context.Context, fn api.Function) error {
	err := ms.Scale(ctx, fn, 0)
	if err != nil {
		return err
	}

	return err
}

// Replicas implements api.MachineService.
func (ms *machineServiceLite) Replicas(_ context.Context, service string) (uint, error) {
	return ms.mStore.ActiveReplicas(service)
}

func (ms *machineServiceLite) machineFromFunction(ctx context.Context, fn api.Function) (machineapi.Machine, error) {
	var (
		limitCpu    resource.Quantity = resource.MustParse("1")
		limitMemory resource.Quantity = resource.MustParse("256Mi")
		reqCpu      resource.Quantity = resource.MustParse("1")
		reqMemory   resource.Quantity = resource.MustParse("256Mi")

		err error
	)

	if fn.Deployment.Limits != nil {
		if lcpu, err := resource.ParseQuantity(fn.Deployment.Limits.CPU); err == nil {
			limitCpu = lcpu
		}
		if lmem, err := resource.ParseQuantity(fn.Deployment.Limits.Memory); err == nil {
			limitMemory = lmem
		}
		if rcpu, err := resource.ParseQuantity(fn.Deployment.Requests.CPU); err == nil {
			reqCpu = rcpu
		}
		if rmem, err := resource.ParseQuantity(fn.Deployment.Requests.Memory); err == nil {
			reqMemory = rmem
		}
	}

	machine := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machineapi.MachineSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    reqCpu,
					"memory": reqMemory,
				},
				Limits: corev1.ResourceList{
					"cpu":    limitCpu,
					"memory": limitMemory,
				},
			},
			Emulation: false,
		},
	}

	machine.Spec.Architecture = fn.Target.Architecture().Name()
	machine.Spec.Platform = fn.Target.Platform().Name()

	machine.ObjectMeta.UID = uuid.NewUUID()
	machine.Status.StateDir = filepath.Join(pkg.MachineDirectory, string(machine.ObjectMeta.UID))
	if err := os.MkdirAll(machine.Status.StateDir, fs.ModeSetgid|0o775); err != nil {
		return machineapi.Machine{}, err
	}

	// Clean up the package directory if an error occurs before returning.
	defer func() {
		if err != nil {
			os.RemoveAll(machine.Status.StateDir)
		}
	}()

	machine.Spec.Kernel = fmt.Sprintf("%s://%s", ms.pm.Format(), fn.Deployment.Image)

	machine.Spec.ApplicationArgs = fn.Target.Command()

	// Setup initrd inside disjointed process
	var ramfs initrd.Initrd
	if fn.Target.Initrd() != nil {
		ramfs = fn.Target.Initrd()
	}
	if ramfs != nil {
		machine.Status.InitrdPath, err = ramfs.Build(ctx)
		if err != nil {
			return machineapi.Machine{}, err
		}
	}

	machine.Status.KernelPath = fn.Target.Kernel()

	if fn.Target.KConfig().AnyYes(
		"CONFIG_LIBVFSCORE_AUTOMOUNT_UP",
	) && (len(machine.Status.InitrdPath) > 0) {
		machine.Spec.Volumes = append(machine.Spec.Volumes, volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fs0",
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      "initrd",
				Destination: "/",
			},
		})
	}
	if fn.Deployment.Annotations != nil {
		machine.ObjectMeta.Annotations = *fn.Deployment.Annotations
	} else {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	if fn.Deployment.Labels != nil {
		machine.ObjectMeta.Labels = *fn.Deployment.Labels
	} else {
		machine.ObjectMeta.Labels = make(map[string]string)
	}
	machine.ObjectMeta.Labels["ukfaas.io/service"] = fn.Deployment.Service
	machine.ObjectMeta.Labels["ukfaas.io/image"] = fn.Deployment.Image
	machine.ObjectMeta.Labels["ukfaas.io/namespace"] = fn.Deployment.Namespace
	machine.Status.State = machineapi.MachineStateCreated

	return *machine, nil
}

func (ms *machineServiceLite) create(ctx context.Context, machine *machineapi.Machine) (*machineapi.Machine, error) {
	// attach Network Device
	// Get Machine Service
	networkSpec, err := ms.attachNetworkDevice(ctx, machine)
	if err != nil {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, err
	}

	log.Printf("machine create called on %s", machine.GetUID())
	machine.Spec.Networks = []networkapi.NetworkSpec{networkSpec}
	machine.ObjectMeta.Name = machinename.NewRandomMachineName(0)
	platform := machine.Spec.Platform

	machineController := ms.machineControllers[mplatform.PlatformByName(platform)]

	machine, err = machineController.Create(ctx, machine)
	if err != nil {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, errors.Wrapf(err, "machine creation for %s failed", platform)
	}
	log.Printf("machine created: %s", machine.GetUID())
	machine, err = machineController.Start(ctx, machine)
	if err != nil {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, errors.Wrapf(err, "machine start for %s failed", platform)
	}
	log.Printf("machine started: %s", machine.GetUID())
	return machine, nil
}

func (ms *machineServiceLite) destroy(ctx context.Context, machine *machineapi.Machine) (*machineapi.Machine, error) {

	platform := machine.Spec.Platform
	machineID := machine.GetUID()

	log.Printf("machine destroy called on %s", machine.GetUID())

	machineStrategy, ok := mplatform.Strategies()[mplatform.PlatformByName(platform)]
	if !ok {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, fmt.Errorf("platform %s not supported", platform)
	}
	machineController, err := machineStrategy.NewMachineV1alpha1(ctx)
	if err != nil {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, err
	}

	oldMachine := machine.DeepCopy()

	machine, err = machineController.Stop(ctx, machine)
	if err != nil {
		log.Printf("[MachineStore.destroyMachine] - Error stopping id:%s err:%v\n", machineID, err)
	}
	log.Printf("machine stopped %s", machine.GetUID())

	machine, err = ms.deleteNetworkDevice(ctx, oldMachine)
	if err != nil {
		machine.Status.State = machineapi.MachineStateErrored
		return machine, err
	}

	if machine == nil {
		return nil, nil
	}

	_, err = machineController.Delete(ctx, machine)
	if err != nil {
		log.Printf("[MachineStore.destroyMachine] - Error deleting id:%s err:%v\n", machineID, err)
	}

	oldMachine.Status.State = machineapi.MachineStateExited
	return oldMachine, nil
}

func (ms *machineServiceLite) attachNetworkDevice(ctx context.Context, machine *machineapi.Machine) (networkapi.NetworkSpec, error) {
	log.Printf("network device create called on %s", machine.GetUID())
	networkName := "openfaas0"
	found, err := ms.networks.Get(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: networkName,
		},
	})
	if err != nil {
		return networkapi.NetworkSpec{}, err
	}

	newIface := networkapi.NetworkInterfaceTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			UID: machine.GetObjectMeta().GetUID(),
		},
		Spec: networkapi.NetworkInterfaceSpec{
			IfName: fmt.Sprintf("%sif%s", networkName, machine.ObjectMeta.UID)[:15],
		},
	}

	if found.Spec.Interfaces == nil {
		found.Spec.Interfaces = []networkapi.NetworkInterfaceTemplateSpec{}
	}
	found.Spec.Interfaces = append(found.Spec.Interfaces, newIface)

	// Update the network with the new interface.
	found, err = ms.networks.Update(ctx, found)
	if err != nil {
		return networkapi.NetworkSpec{}, err
	}

	// Only use the single new interface.
	for _, iface := range found.Spec.Interfaces {
		if iface.UID == newIface.UID {
			newIface = iface
			break
		}
	}
	// Set the interface on the machine.
	found.Spec.IfName = networkName
	found.Spec.Interfaces = []networkapi.NetworkInterfaceTemplateSpec{newIface}
	log.Printf("network device %s created for %s", newIface.Spec.IfName, machine.GetUID())
	return found.Spec, nil
}

func (ms *machineServiceLite) deleteNetworkDevice(_ context.Context, machine *machineapi.Machine) (*machineapi.Machine, error) {
	iface := machine.Spec.Networks[0]
	interfaceName := iface.Interfaces[0].Spec.IfName
	log.Printf("network device delete called on %s for %s", interfaceName, machine.GetUID())

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		log.Printf("ERROR: Could not find %s - %v", iface.IfName, err)
	}

	if link != nil {
		if err := netlink.LinkSetDown(link); err != nil {
			return machine, fmt.Errorf("could not bring %s link down: %v", iface.IfName, err)
		}

		if err := netlink.LinkDel(link); err != nil {
			return machine, fmt.Errorf("could not delete %s link: %v", iface.IfName, err)
		}
	}
	log.Printf("network device %s deleted for %s", iface.IfName, machine.GetUID())
	return machine, nil
}
