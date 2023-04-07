package client_test

import (
	"context"
	"fmt"
	"github.com/massenz/go-statemachine/client"
	"google.golang.org/protobuf/types/known/emptypb"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tc "github.com/testcontainers/testcontainers-go/modules/compose"
)

var (
	svc *client.CliClient
	stack tc.ComposeStack
)
func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Client Suite")
}


func StartServices() {
	ctx := context.Background()

	// Define the Docker Compose setup
	compose, err := tc.NewDockerCompose("../docker/cli-test-compose.yaml")
	Expect(err).ToNot(HaveOccurred())
	stack = compose.WithOsEnv()

	// Start the Docker Compose setup
	Expect(stack.Up(ctx)).To(Succeed())

	// Get the container IP address and port
	smServer, err := stack.ServiceContainer(ctx, "server")
	Expect(err).ToNot(HaveOccurred())
	ip, err := smServer.ContainerIP(ctx)
	Expect(err).ToNot(HaveOccurred())
	port, err := smServer.MappedPort(ctx, "7398")
	Expect(err).ToNot(HaveOccurred())

	svc = client.NewClient(fmt.Sprintf("%s:%s", ip, port.Port()), true)
}

var _ = BeforeSuite(func() {
	StartServices()
	svc = client.NewClient(":7398", true)
	_, err := svc.Health(context.Background(), &emptypb.Empty{})
	Expect(err).Should(Succeed(), "Is the SM Server running? Use 'make start' to start it")
})

var _ = AfterSuite(func() {
	Expect(stack.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)).To(Succeed())
})
