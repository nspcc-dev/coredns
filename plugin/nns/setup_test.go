package nns

import (
	"context"
	"github.com/coredns/caddy"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
	"time"
)

const testImage = "nspccdev/neofs-aio-testcontainer:0.24.0"

func TestSetup(t *testing.T) {
	container := createDockerContainer(context.TODO(), t, testImage)
	defer container.Terminate(context.TODO())

	for _, tc := range []struct {
		endpoint string
		valid    bool
	}{
		{endpoint: "", valid: false},
		{endpoint: "localhost", valid: false},
		{endpoint: "localhost:30333", valid: false},
		{endpoint: "http://localhost", valid: false},
		{endpoint: "http://localhost:30333", valid: true},
	} {
		c := caddy.NewTestController("dns", "nns "+tc.endpoint)
		err := setup(c)
		if tc.valid && err != nil {
			t.Fatalf("Expected no errors, but got: %v", err)
		} else if !tc.valid && err == nil {
			t.Fatalf("Expected errors, but got: %v", err)
		}
	}
}

func createDockerContainer(ctx context.Context, t *testing.T, image string) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:       image,
		WaitingFor:  wait.NewLogStrategy("aio container started").WithStartupTimeout(30 * time.Second),
		Name:        "coredns-aio",
		Hostname:    "coredns-aio",
		NetworkMode: "host",
	}
	aioC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	return aioC
}
