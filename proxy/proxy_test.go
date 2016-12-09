package proxy_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/codeactual/ec2metaproxy/proxy"
	"github.com/pkg/errors"
)

const (
	defaultIP                  = "172.21.0.2"
	ipWithNoLabels             = "172.21.0.3"
	ipWithAllLabels            = "172.21.0.4"
	defaultRoleARNFriendlyName = "NoPerms"
	dbRoleARNFriendlyName      = "SomethingDB"
	defaultPathSpec            = "/"
	defaultPathReqBase         = "/latest/meta-data/iam/security-credentials"
	defaultPathReq             = defaultPathReqBase + "/" + defaultRoleARNFriendlyName
	defaultCustomPolicy        = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["rds:DescribeDBInstances", "rds:DescribeDBClusters"],"Resource":["*"]}]}`
	defaultPolicy              = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:DescribeInstances"],"Resource":["*"]}]}`
	defaultProxiedBody         = "proxied body"
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

func defaultConfig() proxy.Config {
	return proxy.Config{
		AliasToARN: map[string]string{
			"noperms": "arn:aws:iam::123456789012:role/" + defaultRoleARNFriendlyName,
			"db":      "arn:aws:iam::123456789012:role/" + dbRoleARNFriendlyName,
		},
		DefaultAlias: "noperms",
		DockerHost:   "unix:///var/run/alt-docker.sock",
		ListenAddr:   ":20000",
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
	noPermsARN, err := proxy.NewRoleARN(defaultConfig().AliasToARN["noperms"])
	if err != nil {
		panic(fmt.Sprintf("invalid ARN in test fixtures: %+v", err))
	}

	dbARN, err := proxy.NewRoleARN(defaultConfig().AliasToARN["db"])
	if err != nil {
		panic(fmt.Sprintf("invalid ARN in test fixtures: %+v", err))
	}

	return ipContainerInfo{
		defaultIP: proxy.ContainerInfo{
			ID:      "container_0_a975a907324c3d17c92210df4379da3d5964535134a1c42cce580767f615d87d",
			Name:    "container_0_name",
			IamRole: noPermsARN,
		},
		ipWithNoLabels: proxy.ContainerInfo{
			ID:   "container_1_c8edc0715432097101f0e958b61f96412f91fa10e2a29814226cce097dc56b2f",
			Name: "container_1_name",
		},
		ipWithAllLabels: proxy.ContainerInfo{
			ID:        "container_2_30b00758601e903b4a3603bd59bfe15d4d165a33925afe52311f77a8ca02461a",
			Name:      "container_2_name",
			IamRole:   dbARN,
			IamPolicy: defaultCustomPolicy,
		},
	}
}

func defaultContainerSvcStub() *containerServiceStub {
	return newDockerContainerServiceStub(defaultIPContainerInfo())
}

func credsEqualDefaults(t *testing.T, body *bytes.Buffer, stsSvc *assumeRoleStub) {
	var c proxy.MetadataCredentials
	err := json.NewDecoder(body).Decode(&c)
	fatalOnErr(t, err)

	expectedCreds := defaultCreds()
	stringsEqual(t, [][2]string{
		[2]string{"Success", c.Code},
		[2]string{*expectedCreds.AccessKeyId, c.AccessKeyID},
		[2]string{"AWS-HMAC", c.Type},
		[2]string{*expectedCreds.SecretAccessKey, c.SecretAccessKey},
		[2]string{*expectedCreds.SessionToken, c.Token},
		[2]string{stsSvc.output.Credentials.Expiration.String(), c.Expiration.String()},
	})
}

func bodyIsEmpty(t *testing.T, body *bytes.Buffer) {
	bodyBytes, err := ioutil.ReadAll(body)
	fatalOnErr(t, err)
	if len(bodyBytes) != 0 {
		t.Fatalf("expected empty body, got [%s]", string(bodyBytes))
	}
}

func bodyIsNonEmpty(t *testing.T, body *bytes.Buffer) string {
	bodyBytes, err := ioutil.ReadAll(body)
	fatalOnErr(t, err)
	if len(bodyBytes) == 0 {
		t.Fatal("expected non-empty body")
	}
	return string(bodyBytes)
}

func responseCodeIs(t *testing.T, res *httptest.ResponseRecorder, expected int) {
	if res.Code != expected {
		t.Fatalf("expected HTTP code %d, got %d", expected, res.Code)
	}
}

func assumeRoleAliasIsEmpty(t *testing.T, stsSvc *assumeRoleStub) {
	if *stsSvc.input.RoleArn != "" {
		t.Fatalf("expected assume role to be empty, instead [%s]", *stsSvc.input.RoleArn)
	}
}

func assumeRoleAliasIs(t *testing.T, alias string, config proxy.Config, stsSvc *assumeRoleStub) {
	if *stsSvc.input.RoleArn != config.AliasToARN[alias] {
		t.Fatalf("expected assume role to be [%s] with alias [%s], instead [%s]", config.AliasToARN[alias], alias, *stsSvc.input.RoleArn)
	}
}

func assumeRolePolicyIs(t *testing.T, policy string, stsSvc *assumeRoleStub) {
	if *stsSvc.input.Policy != policy {
		t.Fatalf("expected policy [%s], got [%s]", policy, *stsSvc.input.Policy)
	}
}

func assumeRolePolicyIsNil(t *testing.T, stsSvc *assumeRoleStub) {
	if stsSvc.input.Policy != nil {
		t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
	}
}

// StringsEqual asserts all string pairs are equal. In each pair, expected value is first.
func stringsEqual(t *testing.T, pairs [][2]string) {
	for _, v := range pairs {
		if v[0] != v[1] {
			// Use %+v and Errorf to get a stack trace
			t.Fatalf("%+v", errors.Errorf("expected [%s], got [%s]", v[0], v[1]))
		}
	}
}

func fatalOnErr(t *testing.T, err error) {
	if err != nil {
		// Use %+v and Wrap to get a stack trace
		t.Fatalf("%+v", errors.Wrap(err, "unexpected error in test case"))
	}
}
