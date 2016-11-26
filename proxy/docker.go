package proxy

import (
	"context"
	"log"
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
	c, err := client.NewEnvClient()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create docker client with endpoint [%s]", endpoint)
	}
	return &dockerContainerService{
		aliasToARN:     aliasToARN,
		containerIPMap: make(map[string]dockerContainerInfo),
		docker:         c,
		log:            logger,
	}, nil
}

func (d *dockerContainerService) TypeName() string {
	return "docker"
}

func (d *dockerContainerService) ContainerForIP(containerIP string) (containerInfo, error) {
	info, found := d.containerIPMap[containerIP]
	now := time.Now()

	if !found {
		d.syncContainers(now)
		info, found = d.containerIPMap[containerIP]
	} else if now.After(info.RefreshTime) {
		info, found = d.syncContainer(containerIP, info, now)
	}

	if !found {
		return containerInfo{}, errors.Errorf("No container found for IP %s", containerIP)
	}

	return info.containerInfo, nil
}

func (d *dockerContainerService) syncContainer(containerIP string, oldInfo dockerContainerInfo, now time.Time) (dockerContainerInfo, bool) {
	d.log.Printf("Inspecting container: [%s]", oldInfo.ID)
	container, err := d.docker.ContainerInspect(context.Background(), oldInfo.ID)
	if err != nil || container.State.Status != runningState {
		if client.IsErrContainerNotFound(err) {
			d.log.Printf("Container not found, refreshing container info [%s]", oldInfo.ID)
		} else {
			d.log.Printf("Error inspecting container, refreshing container info [%s]: %+v", oldInfo.ID, err)
		}

		d.syncContainers(now)
		info, found := d.containerIPMap[containerIP]
		return info, found
	}

	oldInfo.RefreshTime = refreshTime(now)
	d.containerIPMap[containerIP] = oldInfo
	return oldInfo, true
}

func (d *dockerContainerService) syncContainers(now time.Time) {
	d.log.Printf("Synchronizing state with running docker containers")
	apiContainers, err := d.docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		d.log.Printf("Error listing running containers: %+v", err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]dockerContainerInfo)

	for _, container := range apiContainers {
		if container.State != runningState {
			continue
		}
		alias, ok := container.Labels[LabelKey]
		if !ok {
			continue
		}

		var containerIPs []string
		for netName, net := range container.NetworkSettings.Networks {
			if net.IPAddress != "" {
				containerIPs = append(containerIPs, net.IPAddress)
				d.log.Printf("collectContainerIP: id [%s] name %v net [%s] ip [%s] alias [%s]", container.ID[:10], container.Names, netName, net.IPAddress, alias)
			}
		}

		if len(containerIPs) == 0 {
			d.log.Printf("No IP addresses discovered for container [%s]", container.ID)
			continue
		}

		roleName, ok := d.aliasToARN[alias]
		if !ok {
			d.log.Printf("Container [%s] %v has an unmapped role alias [%s]", container.ID, container.Names, alias)
			continue
		}
		role, roleErr := newRoleArn(roleName)
		if roleErr != nil {
			d.log.Printf("failed to create new role ARN with invalid name [%s]: %+v", role, roleErr)
			continue
		}

		for _, ipAddress := range containerIPs {
			d.log.Printf("Container: id [%s] ip [%s] image [%s] role [%s]", container.ID[:6], ipAddress, container.Image, role)

			containerIPMap[ipAddress] = dockerContainerInfo{
				containerInfo: containerInfo{
					ID:        container.ID,
					Name:      strings.Join(container.Names, ","),
					IamRole:   role,
					IamPolicy: container.Labels[PolicyKey],
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
