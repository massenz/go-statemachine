package client_test

import (
	"context"
	"github.com/massenz/fsm-cli/client"
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

	// The test compose file maps the gRPC server to localhost:7398; connect directly
	// rather than relying on dynamic port mappings from testcontainers.
	svc = client.NewClient("localhost:7398", true)
	Ω(svc).ToNot(BeNil())
}

var _ = BeforeSuite(func() {
	StartServices()
	// Server startup can take a moment after the containers are reported as started,
	// so we poll Health until it succeeds or times out.
	Eventually(func() error {
		_, err := svc.Health(context.Background(), &emptypb.Empty{})
		return err
	}, 30*time.Second, 500*time.Millisecond).Should(Succeed())
})

var _ = AfterSuite(func() {
	Ω(stack.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)).To(Succeed())
})
