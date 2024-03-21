package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/MakeNowJust/heredoc"
	units "github.com/docker/go-units"
	faasd "github.com/openfaas/faasd/pkg"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ukfaaswd stores the directory where all ukfaas binaries and config files are stored
const ukfaaswd = "/var/lib/ukfaasd"

// upConfig are the CLI flags used by the `ukfaas up` command to deploy the openfaas gateway service
type upConfig struct {
	// composeFilePath is the path to the compose file specifying the faasd service configuration
	// See https://compose-spec.io/ for more information about the spec,
	//
	// currently, this must be the name of a file in workingDir, which is set to the value of
	// `ukfaasdwd = /var/lib/ukfaasd`
	composeFilePath string

	// working directory to assume the compose file is in, should be faasdwd.
	// this is not configurable but may be in the future.
	workingDir string
}

func init() {
	configureUpFlags(upCmd.Flags())
}

// The up command starts the openfaas gateway with prometheus. It reuses components from https://github.com/openfaas/faasd
// as both ukfaasd and faasd as single node openfaas providers. In the future, as ukfaasd evolves to support multi-node
// infrasturcture, we will move away from faasd.
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start OpenFaaS gateway",
	Long: heredoc.Docf(`
	ukfaasd requires OpenFaas components such 
	as an OpenFaaS gateway, a promtheues instance
	for monitoring and more additional ones. The
	up command spawns components from the
	docker-compose.yaml as containers.

	More about OpenFaas: https://www.openfaas.com
`),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := parseUpFlags(cmd)
		if err != nil {
			return err
		}

		services, err := loadServiceDefinition(cfg)
		if err != nil {
			return err
		}

		start := time.Now()
		supervisor, err := faasd.NewSupervisor("/run/containerd/containerd.sock")
		if err != nil {
			return err
		}

		log.Printf("Supervisor created in: %s\n", units.HumanDuration(time.Since(start)))

		start = time.Now()
		if err := supervisor.Start(services); err != nil {
			return err
		}
		defer supervisor.Close()

		log.Printf("Supervisor init done in: %s\n", units.HumanDuration(time.Since(start)))

		shutdownTimeout := time.Second * 1
		timeout := time.Second * 60

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

			log.Printf("faasd: waiting for SIGTERM or SIGINT\n")
			<-sig

			log.Printf("Signal received.. shutting down server in %s\n", shutdownTimeout.String())
			err := supervisor.Remove(services)
			if err != nil {
				fmt.Println(err)
			}

			// TODO: close proxies
			time.AfterFunc(shutdownTimeout, func() {
				wg.Done()
			})
		}()

		localResolver := faasd.NewLocalResolver(path.Join(cfg.workingDir, "hosts"))
		go localResolver.Start()

		proxies := map[uint32]*faasd.Proxy{}
		for _, svc := range services {
			for _, port := range svc.Ports {

				listenPort := port.Port
				if _, ok := proxies[listenPort]; ok {
					return fmt.Errorf("port %d already allocated", listenPort)
				}

				hostIP := "0.0.0.0"
				if len(port.HostIP) > 0 {
					hostIP = port.HostIP
				}

				upstream := fmt.Sprintf("%s:%d", svc.Name, port.TargetPort)
				proxies[listenPort] = faasd.NewProxy(upstream, listenPort, hostIP, timeout, localResolver)
			}
		}

		// track proxies for later cancellation when receiving sigint/term
		for _, v := range proxies {
			go v.Start()
		}

		wg.Wait()
		return nil
	},
}

// load the docker compose file and then parse it as supervisor Services
// the logic for loading the compose file comes from the compose reference implementation
// https://github.com/compose-spec/compose-ref/blob/master/compose-ref.go#L353
func loadServiceDefinition(cfg upConfig) ([]faasd.Service, error) {

	serviceConfig, err := faasd.LoadComposeFile(cfg.workingDir, cfg.composeFilePath)
	if err != nil {
		return nil, err
	}

	return faasd.ParseCompose(serviceConfig)
}

// ConfigureUpFlags will define the flags for the `ukfaasd up` command. The flag struct, configure, and
// parse are split like this to simplify testability.
func configureUpFlags(flags *pflag.FlagSet) {
	flags.StringP("file", "f", "docker-compose.yaml", "compose file specifying the faasd service configuration")
}

// ParseUpFlags will load the flag values into an upFlags object. Errors will be underlying
// Get errors from the pflag library.
func parseUpFlags(cmd *cobra.Command) (upConfig, error) {
	currDir, err := os.Getwd()
	if err != nil {
		currDir = ukfaaswd
	}

	parsed := upConfig{
		workingDir: currDir,
	}
	path, err := cmd.Flags().GetString("file")
	if err != nil {
		return parsed, errors.Wrap(err, "can not parse compose file path flag")
	}

	parsed.composeFilePath = path
	return parsed, err
}
