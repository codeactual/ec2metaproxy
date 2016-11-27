package proxy

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

const runningState = "running"

type dockerContainerInfo struct {
	containerInfo
	RefreshTime time.Time
}

type dockerContainerService struct {
	containerIPMap map[string]dockerContainerInfo
	aliasToARN     map[string]string
	docker         *client.Client
	log            *log.Logger
}

func newDockerContainerService(endpoint string, aliasToARN map[string]string, logger *log.Logger) (*dockerContainerService, error) {
	os.Setenv("DOCKER_HOST", endpoint)

	c, err := client.NewEnvClient()
	if err != nil {
		return nil, errors.Wrapf(err, "Error creating docker client with endpoint [%s]", endpoint)
	}
	return &dockerContainerService{
		aliasToARN:     aliasToARN,
		containerIPMap: make(map[string]dockerContainerInfo),
		docker:         c,
		log:            logger,
	}, nil
}

// TypeName implements a containerService method.
func (d *dockerContainerService) TypeName() string {
	return "docker"
}

// ContainerForIP implements a containerService method.
//
// If containerInfo exists in the cache, keyed by the container IP, then it is returned.
// Otherwise syncContainer is used to collect fresh containerInfo from the docker API.
func (d *dockerContainerService) ContainerForIP(ctx context.Context, containerIP string) (containerInfo, error) {
	info, found := d.containerIPMap[containerIP]
	now := time.Now()

	if !found {
		d.syncContainers(ctx, now)
		info, found = d.containerIPMap[containerIP]
	} else if now.After(info.RefreshTime) {
		info, found = d.syncContainer(ctx, containerIP, info, now)
	}

	if !found {
		return containerInfo{}, errors.Errorf("No container found for IP %s", containerIP)
	}

	return info.containerInfo, nil
}

func (d *dockerContainerService) syncContainer(ctx context.Context, containerIP string, oldInfo dockerContainerInfo, now time.Time) (dockerContainerInfo, bool) {
	reqID := requestIDFromContext(ctx)

	container, err := d.docker.ContainerInspect(ctx, oldInfo.ID)

	if err != nil || container.State.Status != runningState {
		if client.IsErrContainerNotFound(err) {
			d.log.Printf("syncContainer (%s): container not found, refreshing container info [%s]", reqID, oldInfo.ID)
		} else {
			d.log.Printf("syncContainer (%s): Error inspecting container, refreshing container info [%s]: %+v", reqID, oldInfo.ID, err)
		}

		d.syncContainers(ctx, now)
		info, found := d.containerIPMap[containerIP]
		return info, found
	}

	oldInfo.RefreshTime = refreshTime(now)
	d.containerIPMap[containerIP] = oldInfo
	return oldInfo, true
}

func (d *dockerContainerService) syncContainers(ctx context.Context, now time.Time) {
	reqID := requestIDFromContext(ctx)

	apiContainers, err := d.docker.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		d.log.Printf("syncContainers (%s): Error listing running containers: %+v", reqID, err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]dockerContainerInfo)

	for _, container := range apiContainers {
		if container.State != runningState {
			continue
		}
		alias, ok := container.Labels[RoleLabelKey]
		if !ok {
			continue
		}

		var containerIPs []string
		for _, net := range container.NetworkSettings.Networks {
			if net.IPAddress != "" {
				containerIPs = append(containerIPs, net.IPAddress)
			}
		}

		if len(containerIPs) == 0 {
			d.log.Printf("syncContainers (%s): no IP addresses discovered for container [%s]", reqID, container.ID)
			continue
		}

		roleName, ok := d.aliasToARN[alias]
		if !ok {
			d.log.Printf("syncContainers (%s): container [%s] %v has an unmapped role alias [%s]", reqID, container.ID, container.Names, alias)
			continue
		}
		role, roleErr := newRoleArn(roleName)
		if roleErr != nil {
			d.log.Printf("syncContainers (%s): Error creating new role ARN with invalid name [%s]: %+v", reqID, role, roleErr)
			continue
		}

		for _, ipAddress := range containerIPs {
			d.log.Printf("syncContainers (%s): id [%s] ip [%s] image [%s] role [%s]", reqID, container.ID[:6], ipAddress, container.Image, role)

			containerIPMap[ipAddress] = dockerContainerInfo{
				containerInfo: containerInfo{
					ID:        container.ID,
					Name:      strings.Join(container.Names, ","),
					IamRole:   role,
					IamPolicy: container.Labels[PolicyLabelKey],
				},
				RefreshTime: refreshAt,
			}
		}
	}

	d.containerIPMap = containerIPMap
}

func refreshTime(now time.Time) time.Time {
	return now.Add(1 * time.Second)
}
