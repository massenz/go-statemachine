package client_test

import (
	"context"
	"fmt"
	"github.com/massenz/go-statemachine/client"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tc "github.com/testcontainers/testcontainers-go/modules/compose"
)

var (
	svc   *client.CliClient
	stack tc.ComposeStack
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Client Suite")
}

func StartServices() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	composeYaml := os.Getenv("CLI_TEST_COMPOSE")
	Ω(composeYaml).ShouldNot(BeEmpty())

	// Define the Docker Compose setup
	//
	// If the tests fail because the test server is not coming up, you can
	// debug (and see logs) using this command:
	//   RELEASE=$(make version) BASEDIR=$(pwd) docker compose -f docker/cli-test-compose.yaml up
	compose, err := tc.NewDockerCompose(composeYaml)
	Ω(err).ToNot(HaveOccurred())
	stack = compose.WithOsEnv()

	// Start the Docker Compose setup
	Ω(stack.Up(ctx)).To(Succeed())

	// Get the container IP address and port
	smServer, err := stack.ServiceContainer(ctx, "server")
	Ω(err).ToNot(HaveOccurred())
	port, err := smServer.MappedPort(ctx, "7398")
	Ω(err).ToNot(HaveOccurred())

	// It is *important* to use `localhost` here, as Certs are issued with that hostname
	svc = client.NewClient(fmt.Sprintf("localhost:%s", port.Port()), true)
	Ω(svc).ToNot(BeNil())
}

var _ = BeforeSuite(func() {
	StartServices()
	_, err := svc.Health(context.Background(), &emptypb.Empty{})
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	Ω(stack.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)).To(Succeed())
})
