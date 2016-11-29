package proxy

import (
	"context"

	"github.com/pkg/errors"
)

// ipContainerInfo maps IPs to containerInfo fields.
type ipContainerInfo map[string]containerInfo

// containerServiceStub queries its ContainerInfo map instead of the Docker daemon.
type containerServiceStub struct {
	info ipContainerInfo
}

func (c *containerServiceStub) ContainerForIP(ctx context.Context, containerIP string) (containerInfo, error) {
	i, ok := c.info[containerIP]
	if !ok {
		return i, errors.Errorf("No container found for IP [%s]", containerIP)
	}
	return i, nil
}

func (c *containerServiceStub) TypeName() string {
	return "docker"
}

// newDockerContainerServiceStub creates a service stub backed by the chosen info.
func newDockerContainerServiceStub(info ipContainerInfo) *containerServiceStub {
	return &containerServiceStub{info: info}
}
