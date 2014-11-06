package main

import (
	"encoding/json"
	"flag"
	"net"
	"net/url"
	"os"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/diegonats"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/stager/cc_client"
	"github.com/cloudfoundry-incubator/stager/inbox"
	"github.com/cloudfoundry-incubator/stager/outbox"
	"github.com/cloudfoundry-incubator/stager/stager"
	"github.com/cloudfoundry-incubator/stager/stager_docker"
)

var natsAddresses = flag.String(
	"natsAddresses",
	"",
	"comma-separated list of NATS addresses (ip:port)",
)

var natsUsername = flag.String(
	"natsUsername",
	"",
	"Username to connect to nats",
)

var natsPassword = flag.String(
	"natsPassword",
	"",
	"Password for nats user",
)

var ccBaseURL = flag.String(
	"ccBaseURL",
	"",
	"URI to acccess the Cloud Controller",
)

var ccUsername = flag.String(
	"ccUsername",
	"",
	"Basic auth username for CC internal API",
)

var ccPassword = flag.String(
	"ccPassword",
	"",
	"Basic auth password for CC internal API",
)

var skipCertVerify = flag.Bool(
	"skipCertVerify",
	false,
	"skip SSL certificate verification",
)

var circuses = flag.String(
	"circuses",
	"{}",
	"Map of circuses for different stacks (name => compiler_name)",
)

var dockerCircusPath = flag.String(
	"dockerCircusPath",
	"",
	"path for downloading docker circus from file server",
)

var minMemoryMB = flag.Uint(
	"minMemoryMB",
	1024,
	"minimum memory limit for staging tasks",
)

var minDiskMB = flag.Uint(
	"minDiskMB",
	3072,
	"minimum disk limit for staging tasks",
)

var minFileDescriptors = flag.Uint64(
	"minFileDescriptors",
	0,
	"minimum file descriptors for staging tasks",
)

var diegoAPIURL = flag.String(
	"diegoAPIURL",
	"",
	"URL of diego API",
)

var stagerURL = flag.String(
	"stagerURL",
	"",
	"URL of the stager",
)

var fileServerURL = flag.String(
	"fileServerURL",
	"",
	"URL of the file server",
)

var dropsondeOrigin = flag.String(
	"dropsondeOrigin",
	"stager",
	"Origin identifier for dropsonde-emitted metrics.",
)

var dropsondeDestination = flag.String(
	"dropsondeDestination",
	"localhost:3457",
	"Destination for dropsonde-emitted metrics.",
)

func main() {
	flag.Parse()

	logger := cf_lager.New("stager")
	initializeDropsonde(logger)
	traditionalStager, dockerStager := initializeStagers(logger)
	ccClient := cc_client.NewCcClient(*ccBaseURL, *ccUsername, *ccPassword, *skipCertVerify)

	cf_debug_server.Run()

	natsClient := diegonats.NewClient()

	address, err := getStagerAddress()
	if err != nil {
		logger.Fatal("Invalid stager URL", err)
	}

	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{"nats", diegonats.NewClientRunner(*natsAddresses, *natsUsername, *natsPassword, logger, natsClient)},
		{"inbox", ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			return inbox.New(natsClient, ccClient, traditionalStager, dockerStager, inbox.ValidateRequest, logger).Run(signals, ready)
		})},
		{"outbox", outbox.New(address, ccClient, logger, timeprovider.NewTimeProvider())},
	})

	process := ifrit.Envoke(sigmon.New(group))

	logger.Info("Listening for staging requests!")

	err = <-process.Wait()
	if err != nil {
		logger.Fatal("Stager exited with error", err)
	}
}

func initializeDropsonde(logger lager.Logger) {
	err := dropsonde.Initialize(*dropsondeOrigin, *dropsondeDestination)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeStagers(logger lager.Logger) (stager.Stager, stager_docker.DockerStager) {
	circusesMap := make(map[string]string)
	err := json.Unmarshal([]byte(*circuses), &circusesMap)
	if err != nil {
		logger.Fatal("Error parsing circuses flag", err)
	}
	config := stager.Config{
		CallbackURL:        *stagerURL,
		FileServerURL:      *fileServerURL,
		Circuses:           circusesMap,
		DockerCircusPath:   *dockerCircusPath,
		MinMemoryMB:        *minMemoryMB,
		MinDiskMB:          *minDiskMB,
		MinFileDescriptors: *minFileDescriptors,
	}

	diegoAPIClient := receptor.NewClient(*diegoAPIURL, "", "")

	bpStager := stager.New(diegoAPIClient, logger, config)
	dockerStager := stager_docker.New(diegoAPIClient, logger, config)

	return bpStager, dockerStager
}

func getStagerAddress() (string, error) {
	url, err := url.Parse(*stagerURL)
	if err != nil {
		return "", err
	}

	_, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		return "", err
	}

	return "0.0.0.0:" + port, nil
}