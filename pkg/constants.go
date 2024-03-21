package pkg

const (
	// DefaultFunctionNamespace is the default containerd namespace functions are created
	DefaultFunctionNamespace = "openfaas-fn"

	// NamespaceLabel indicates that a namespace is managed by ukfaasd
	NamespaceLabel = "openfaas"

	// ukfaasNamespace is the containerd namespace services are created
	DefaultContainerdNamespace = "openfaas"

	faasServicesPullAlways = false

	defaultSnapshotter = "overlayfs"

	OCIDirectory     = "/tmp/kraftkit/oci"
	MachineDirectory = "/tmp/kraftkit/machines"

	ServiceIPSubnet = "10.63.0.0/16"

	WatchdogPort = 8080
	GatewayPort  = 80
)
