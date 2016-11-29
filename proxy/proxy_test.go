package proxy

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
)

const (
	defaultPathSpec     = "/"
	defaultPathReqBase  = "/latest/meta-data/iam/security-credentials"
	defaultPathReq      = defaultPathReqBase + "/NoPerms"
	defaultCustomPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["rds:DescribeDBInstances", "rds:DescribeDBClusters"],"Resource":["*"]}]}`
)

func defaultConfig() Config {
	return Config{
		AliasToARN: map[string]string{
			"noperms": "arn:aws:iam::123456789012:role/NoPerms",
			"db":      "arn:aws:iam::123456789012:role/SomethingDB",
		},
		DefaultAlias:  "noperms",
		DefaultPolicy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:DescribeInstances"],"Resource":["*"]}]}`,
		DockerHost:    "unix:///var/run/alt-docker.sock",
		ListenAddr:    ":20000",
	}
}

func defaultCreds() *sts.Credentials {
	return &sts.Credentials{
		AccessKeyId:     aws.String("fakeAccessKeyId"),
		Expiration:      aws.Time(time.Now().Add(900 * time.Second)),
		SecretAccessKey: aws.String("fakeSecretAccessKey"),
		SessionToken:    aws.String("fakeSessionToken"),
	}
}

func defaultAssumeRoleOutput() *sts.AssumeRoleOutput {
	return &sts.AssumeRoleOutput{Credentials: defaultCreds()}
}

func defaultStsSvcStub() *assumeRoleStub {
	return newAssumeRoleStubReturns(defaultAssumeRoleOutput(), nil)
}

func defaultIPContainerInfo() ipContainerInfo {
	arn, err := newRoleArn(defaultConfig().AliasToARN["db"])
	if err != nil {
		panic(fmt.Sprintf("invalid ARN in test fixtures: %+v", err))
	}

	return ipContainerInfo{
		"172.17.0.2": containerInfo{
			ID:        "container_0_id",
			Name:      "container_0_name",
			IamRole:   arn,
			IamPolicy: defaultCustomPolicy,
		},
		"172.17.0.3": containerInfo{
			ID:   "container_1_id",
			Name: "container_1_name",
		},
	}
}

func defaultContainerSvcStub() *containerServiceStub {
	return newDockerContainerServiceStub(defaultIPContainerInfo())
}

func TestRoleLabel(t *testing.T) {
	t.Run("unknown alias", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, logs, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc)
		fatalOnErr(t, err)
	})

	t.Run("empty alias", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("valid alias", func(t *testing.T) {
		t.Skip("TODO")
	})
}
