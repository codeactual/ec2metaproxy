package proxy

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"testing"
)

func TestResponse(t *testing.T) {
	t.Run("should support alias match with request path", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)
		credsEqualDefaults(t, res.Body, stsSvc)
		assumeRoleAliasIs(t, "noperms", config, stsSvc)
		assumeRolePolicyIsNil(t, stsSvc)
	})

	t.Run("should detect alias mismatch with request path", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/invalid", config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 404)
		bodyIsEmpty(t, res.Body)
		assumeRoleAliasIs(t, "noperms", config, stsSvc)
		assumeRolePolicyIsNil(t, stsSvc)
	})

	t.Run("should detect request path without role", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/", config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)
		body := bodyIsNonEmpty(t, res.Body)

		if body != defaultRoleARNFriendlyName {
			t.Fatalf("expected role ARN [%s], got [%s]", defaultRoleARNFriendlyName, body)
		}

		assumeRoleAliasIs(t, "noperms", config, stsSvc)
		assumeRolePolicyIsNil(t, stsSvc)
	})

	t.Run("should apply default role", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)
		credsEqualDefaults(t, res.Body, stsSvc)
		assumeRoleAliasIs(t, "noperms", config, stsSvc)
		assumeRolePolicyIsNil(t, stsSvc)
	})

	t.Run("should apply default policy", func(t *testing.T) {
		config := defaultConfig()
		config.DefaultPolicy = defaultPolicy

		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)
		credsEqualDefaults(t, res.Body, stsSvc)
		assumeRoleAliasIs(t, "noperms", config, stsSvc)
		assumeRolePolicyIs(t, defaultPolicy, stsSvc)
	})

	t.Run("should support custom labels", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/"+dbRoleARNFriendlyName, config, stsSvc, containerSvc, ipWithAllLabels)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)
		credsEqualDefaults(t, res.Body, stsSvc)
		assumeRoleAliasIs(t, "db", config, stsSvc)
		assumeRolePolicyIs(t, defaultCustomPolicy, stsSvc)
	})

	t.Run("should support no selected defaults", func(t *testing.T) {
		config := defaultConfig()
		config.DefaultAlias = ""

		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 404)
		bodyIsEmpty(t, res.Body)
		assumeRoleAliasIsEmpty(t, stsSvc)
		assumeRolePolicyIsNil(t, stsSvc)
	})
}

func credsEqualDefaults(t *testing.T, body *bytes.Buffer, stsSvc *assumeRoleStub) {
	var c metadataCredentials
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

func assumeRoleAliasIs(t *testing.T, alias string, config Config, stsSvc *assumeRoleStub) {
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
