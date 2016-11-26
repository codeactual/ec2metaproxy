package proxy

import "context"

type containerInfo struct {
	ID        string
	Name      string
	IamRole   roleArn
	IamPolicy string
}

type containerService interface {
	ContainerForIP(ctx context.Context, containerIP string) (containerInfo, error)
	TypeName() string
}
