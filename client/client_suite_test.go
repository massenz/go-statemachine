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
	//
	// If the tests fail because the test server is not coming up, you can
	// debug (and see logs) using this command:
	//   RELEASE=$(make version) BASEDIR=$(pwd) docker compose -f docker/cli-test-compose.yaml up
	compose, err := tc.NewDockerCompose("../docker/cli-test-compose.yaml")
	Expect(err).ToNot(HaveOccurred())
	stack = compose.WithOsEnv()

	// Start the Docker Compose setup
	Expect(stack.Up(ctx)).To(Succeed())

	// Get the container IP address and port
	smServer, err := stack.ServiceContainer(ctx, "server")
	Expect(err).ToNot(HaveOccurred())
	port, err := smServer.MappedPort(ctx, "7398")
	Expect(err).ToNot(HaveOccurred())

	// It is *important* to use `localhost` here, as Certs are issued with that hostname
	svc = client.NewClient(fmt.Sprintf("localhost:%s", port.Port()), true)
	Expect(svc).ToNot(BeNil())
}

var _ = BeforeSuite(func() {
	StartServices()
	_, err := svc.Health(context.Background(), &emptypb.Empty{})
	Expect(err).Should(Succeed())
})

var _ = AfterSuite(func() {
	Expect(stack.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)).To(Succeed())
})
