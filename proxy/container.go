package proxy

import "context"

// ContainerInfo can identify a specific container and its IAM role/policy.
type ContainerInfo struct {
	ID        string
	Name      string
	IamRole   RoleARN
	IamPolicy string
}

// ContainerService implementations provide ContainerInfo.
type ContainerService interface {
	ContainerForIP(ctx context.Context, containerIP string) (ContainerInfo, error)
	TypeName() string
}
