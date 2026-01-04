package client_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/massenz/fsm-cli/client"
	"google.golang.org/protobuf/types/known/emptypb"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	insecureClient       *client.CliClient
	tlsClient            *client.CliClient
	redisContainer       testcontainers.Container
	insecureServerContainer testcontainers.Container
	tlsServerContainer      testcontainers.Container
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Client Suite")
}

func StartServices() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Start Redis container
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:6.2-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	var err error
	redisContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	Ω(err).ShouldNot(HaveOccurred())

	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	Ω(err).ShouldNot(HaveOccurred())
	// The CLI tests run from the host, while the server and Redis run in
	// containers. The Redis container publishes 6379 on a random host port;
	// from inside the server containers we can reach it via the host-mapped
	// port using the special DNS name `host.docker.internal`.
	redisAddr := fmt.Sprintf("host.docker.internal:%s", redisPort.Port())

	// Build the image reference from the same name/tag as `make container`.
	release := os.Getenv("RELEASE")
	Ω(release).ShouldNot(BeEmpty())
	image := fmt.Sprintf("massenz/statemachine:%s", release)

	baseDir := os.Getenv("BASEDIR")
	Ω(baseDir).ShouldNot(BeEmpty())
	certsDir := filepath.Join(baseDir, "certs")

	// Insecure server (no TLS)
	insecureReq := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"7399/tcp"},
		Env: map[string]string{
			"REDIS":          redisAddr,
			"GRPC_PORT":      "7399",
			"INSECURE":       "-insecure",
			"DEBUG":          "-debug",
			"EVENTS_Q":       "",
			"NOTIFICATIONS_Q": "",
		},
		WaitingFor: wait.ForListeningPort("7399/tcp").WithStartupTimeout(2 * time.Minute),
	}

	insecureServerContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: insecureReq,
		Started:          true,
	})
	Ω(err).ShouldNot(HaveOccurred())

	hostInsecure, err := insecureServerContainer.Host(ctx)
	Ω(err).ShouldNot(HaveOccurred())
	mappedPortInsecure, err := insecureServerContainer.MappedPort(ctx, "7399/tcp")
	Ω(err).ShouldNot(HaveOccurred())
	addressInsecure := fmt.Sprintf("%s:%s", hostInsecure, mappedPortInsecure.Port())

	insecureClient = client.NewClient(addressInsecure, false)
	Ω(insecureClient).ShouldNot(BeNil())

	// TLS-enabled server
	tlsReq := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"7398/tcp"},
		Env: map[string]string{
			"REDIS":          redisAddr,
			"GRPC_PORT":      "7398",
			"DEBUG":          "-debug",
			"EVENTS_Q":       "",
			"NOTIFICATIONS_Q": "",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds, fmt.Sprintf("%s:/etc/statemachine/certs:ro", certsDir))
		},
		WaitingFor: wait.ForListeningPort("7398/tcp").WithStartupTimeout(2 * time.Minute),
	}

	tlsServerContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: tlsReq,
		Started:          true,
	})
	Ω(err).ShouldNot(HaveOccurred())

	hostTLS, err := tlsServerContainer.Host(ctx)
	Ω(err).ShouldNot(HaveOccurred())
	mappedPortTLS, err := tlsServerContainer.MappedPort(ctx, "7398/tcp")
	Ω(err).ShouldNot(HaveOccurred())
	addressTLS := fmt.Sprintf("%s:%s", hostTLS, mappedPortTLS.Port())

	// Connect to the servers. The insecure client disables TLS, while the TLS
	// client validates the server certificate using ~/.fsm/certs/ca.pem.
	tlsClient = client.NewClient(addressTLS, true)
	Ω(tlsClient).ShouldNot(BeNil())
}

var _ = BeforeSuite(func() {
	StartServices()
	// Server startup can take a moment after the containers are reported as started,
	// so we poll Health for both insecure and TLS servers until it succeeds or times out.
	Eventually(func() error {
		_, err := insecureClient.Health(context.Background(), &emptypb.Empty{})
		return err
	}, 30*time.Second, 500*time.Millisecond).Should(Succeed())
	Eventually(func() error {
		_, err := tlsClient.Health(context.Background(), &emptypb.Empty{})
		return err
	}, 30*time.Second, 500*time.Millisecond).Should(Succeed())
})

var _ = AfterSuite(func() {
	ctx := context.Background()
	if tlsServerContainer != nil {
		Ω(tlsServerContainer.Terminate(ctx)).To(Succeed())
	}
	if insecureServerContainer != nil {
		Ω(insecureServerContainer.Terminate(ctx)).To(Succeed())
	}
	if redisContainer != nil {
		Ω(redisContainer.Terminate(ctx)).To(Succeed())
	}
})
