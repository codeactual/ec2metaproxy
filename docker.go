package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const runningState = "running"

type dockerContainerInfo struct {
	containerInfo
	RefreshTime time.Time
}

type dockerContainerService struct {
	containerIPMap map[string]dockerContainerInfo
	docker         *client.Client
}

func newDockerContainerService(endpoint string) (*dockerContainerService, error) {
	c, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	if err != nil {
		return nil, err
	}

	return &dockerContainerService{
		containerIPMap: make(map[string]dockerContainerInfo),
		docker:         c,
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
		return containerInfo{}, fmt.Errorf("No container found for IP %s", containerIP)
	}

	return info.containerInfo, nil
}

func (d *dockerContainerService) syncContainer(containerIP string, oldInfo dockerContainerInfo, now time.Time) (dockerContainerInfo, bool) {
	verbosef("Inspecting container: [%s]", oldInfo.ID)
	container, err := d.docker.ContainerInspect(context.Background(), oldInfo.ID)
	if err != nil || container.State.Status != runningState {
		if client.IsErrContainerNotFound(err) {
			verbosef("Container not found, refreshing container info [%s]", oldInfo.ID)
		} else {
			verbosef("Error inspecting container, refreshing container info [%s]: %+v", oldInfo.ID, err)
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
	verbosef("Synchronizing state with running docker containers")
	apiContainers, err := d.docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		verbosef("Error listing running containers: %=v", err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]dockerContainerInfo)

	for _, container := range apiContainers {
		if container.State != runningState {
			continue
		}
		alias, ok := container.Labels[labelKey]
		if !ok {
			continue
		}

		var containerIPs []string
		for netName, net := range container.NetworkSettings.Networks {
			if net.IPAddress != "" {
				containerIPs = append(containerIPs, net.IPAddress)
				verbosef("collectContainerIP: id [%s] name %v net [%s] ip [%s] alias [%s]\n", container.ID[:10], container.Names, netName, net.IPAddress, alias)
			}
		}

		if len(containerIPs) == 0 {
			verbosef("No IP addresses discovered for container [%s]", container.ID)
			continue
		}

		roleName, ok := proxyConfig.AliasToARN[alias]
		if !ok {
			verbosef("Container [%s] %v has an unmapped role alias [%s]", container.ID, container.Names, alias)
			continue
		}
		role, roleErr := newRoleArn(roleName)
		if roleErr != nil {
			verbosef("failed to create new role ARN with invalid name [%s]: %+v", role, roleErr)
			continue
		}

		for _, ipAddress := range containerIPs {
			verbosef("Container: id [%s] ip [%s] image [%s] role [%s]", container.ID[:6], ipAddress, container.Image, role)

			containerIPMap[ipAddress] = dockerContainerInfo{
				containerInfo: containerInfo{
					ID:        container.ID,
					Name:      strings.Join(container.Names, ","),
					IamRole:   role,
					IamPolicy: container.Labels[policyKey],
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
