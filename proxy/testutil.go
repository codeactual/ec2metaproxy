package proxy

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

const (
	defaultPathSpec     = "/"
	defaultPathReqBase  = "/latest/meta-data/iam/security-credentials"
	defaultPathReq      = defaultPathReqBase + "/NoPerms"
	defaultCustomPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["rds:DescribeDBInstances", "rds:DescribeDBClusters"],"Resource":["*"]}]}`
)

type assumeRoleFn func(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)

// assumeRoleStub can be passed to proxy.New to avoid real AWS requests, control AssumeRole behavior,
// and record input.
type assumeRoleStub struct {
	stsiface.STSAPI
	fn     assumeRoleFn
	input  *sts.AssumeRoleInput
	output *sts.AssumeRoleOutput
	err    error
}

// AssumeROle records input and returns configured output/error.
func (s *assumeRoleStub) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	s.input = input
	output, err := s.fn(input)
	s.output = output
	s.err = err
	return s.output, s.err
}

func newAssumeRoleStubReturns(output *sts.AssumeRoleOutput, err error) *assumeRoleStub {
	return &assumeRoleStub{
		fn: func(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
			return output, err
		},
	}
}

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

func defaultIP() string {
	return "172.21.0.2"
}

func defaultIPContainerInfo() ipContainerInfo {
	arn, err := newRoleArn(defaultConfig().AliasToARN["noperms"])
	if err != nil {
		panic(fmt.Sprintf("invalid ARN in test fixtures: %+v", err))
	}

	return ipContainerInfo{
		defaultIP(): containerInfo{
			ID:        "container_0_a975a907324c3d17c92210df4379da3d5964535134a1c42cce580767f615d87d",
			Name:      "container_0_name",
			IamRole:   arn,
			IamPolicy: defaultCustomPolicy,
		},
		"172.17.0.3": containerInfo{
			ID:   "container_1_c8edc0715432097101f0e958b61f96412f91fa10e2a29814226cce097dc56b2f",
			Name: "container_1_name",
		},
	}
}

func defaultContainerSvcStub() *containerServiceStub {
	return newDockerContainerServiceStub(defaultIPContainerInfo())
}

// StringsEqual asserts all string pairs are equal. In each pair, expected value is first.
func stringsEqual(t *testing.T, pairs [][2]string) {
	for _, v := range pairs {
		if v[0] != v[1] {
			t.Fatalf("expected [%s], got [%s]", v[0], v[1])
		}
	}
}

func fatalOnErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
}
