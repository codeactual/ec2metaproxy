package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestResponse(t *testing.T) {
	t.Run("should support alias match with request path", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		if res.Code != 200 {
			t.Fatalf("expected HTTP code 200, got %d", res.Code)
		}

		var body metadataCredentials
		err = json.NewDecoder(res.Body).Decode(&body)
		fatalOnErr(t, err)

		expectedCreds := defaultCreds()
		stringsEqual(t, [][2]string{
			[2]string{"Success", body.Code},
			[2]string{*expectedCreds.AccessKeyId, body.AccessKeyID},
			[2]string{"AWS-HMAC", body.Type},
			[2]string{*expectedCreds.SecretAccessKey, body.SecretAccessKey},
			[2]string{*expectedCreds.SessionToken, body.Token},
			[2]string{stsSvc.output.Credentials.Expiration.String(), body.Expiration.String()},
		})

		if *stsSvc.input.RoleArn != config.AliasToARN["noperms"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["noperms"], *stsSvc.input.RoleArn)
		}
		if stsSvc.input.Policy != nil {
			t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
		}
	})

	t.Run("should detect alias mismatch with request path", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/invalid", config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		if res.Code != 404 {
			t.Fatalf("expected HTTP code 404, got %d", res.Code)
		}

		bodyBytes, err := ioutil.ReadAll(res.Body)
		fatalOnErr(t, err)
		if len(bodyBytes) != 0 {
			t.Fatalf("expected empty body, got [%s]", string(bodyBytes))
		}

		if *stsSvc.input.RoleArn != config.AliasToARN["noperms"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["noperms"], *stsSvc.input.RoleArn)
		}
		if stsSvc.input.Policy != nil {
			t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
		}
	})

	t.Run("should detect request path without role", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/", config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		if res.Code != 200 {
			t.Fatalf("expected HTTP code 200, got %d", res.Code)
		}

		bodyBytes, err := ioutil.ReadAll(res.Body)
		fatalOnErr(t, err)
		if len(bodyBytes) == 0 {
			t.Fatal("expected non-empty body")
		}
		bodyBytesStr := string(bodyBytes)

		if bodyBytesStr != defaultRoleARNFriendlyName {
			t.Fatalf("expected role ARN [%s], got [%s]", defaultRoleARNFriendlyName, bodyBytesStr)
		}

		if *stsSvc.input.RoleArn != config.AliasToARN["noperms"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["noperms"], *stsSvc.input.RoleArn)
		}
		if stsSvc.input.Policy != nil {
			t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
		}
	})

	t.Run("should apply default role", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		if res.Code != 200 {
			t.Fatalf("expected HTTP code 200, got %d", res.Code)
		}

		var body metadataCredentials
		err = json.NewDecoder(res.Body).Decode(&body)
		fatalOnErr(t, err)

		expectedCreds := defaultCreds()
		stringsEqual(t, [][2]string{
			[2]string{"Success", body.Code},
			[2]string{*expectedCreds.AccessKeyId, body.AccessKeyID},
			[2]string{"AWS-HMAC", body.Type},
			[2]string{*expectedCreds.SecretAccessKey, body.SecretAccessKey},
			[2]string{*expectedCreds.SessionToken, body.Token},
			[2]string{stsSvc.output.Credentials.Expiration.String(), body.Expiration.String()},
		})

		if *stsSvc.input.RoleArn != config.AliasToARN["noperms"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["noperms"], *stsSvc.input.RoleArn)
		}
		if stsSvc.input.Policy != nil {
			t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
		}
	})

	t.Run("should apply default policy", func(t *testing.T) {
		config := defaultConfig()
		config.DefaultPolicy = defaultPolicy

		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		if res.Code != 200 {
			t.Fatalf("expected HTTP code 200, got %d", res.Code)
		}

		var body metadataCredentials
		err = json.NewDecoder(res.Body).Decode(&body)
		fatalOnErr(t, err)

		expectedCreds := defaultCreds()
		stringsEqual(t, [][2]string{
			[2]string{"Success", body.Code},
			[2]string{*expectedCreds.AccessKeyId, body.AccessKeyID},
			[2]string{"AWS-HMAC", body.Type},
			[2]string{*expectedCreds.SecretAccessKey, body.SecretAccessKey},
			[2]string{*expectedCreds.SessionToken, body.Token},
			[2]string{stsSvc.output.Credentials.Expiration.String(), body.Expiration.String()},
		})

		if *stsSvc.input.RoleArn != config.AliasToARN["noperms"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["noperms"], *stsSvc.input.RoleArn)
		}
		if *stsSvc.input.Policy != defaultPolicy {
			t.Fatalf("expected custom policy [%s], got [%s]", defaultPolicy, *stsSvc.input.Policy)
		}
	})

	t.Run("should support custom labels", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReqBase+"/"+dbRoleARNFriendlyName, config, stsSvc, containerSvc, ipWithAllLabels)
		fatalOnErr(t, err)

		if res.Code != 200 {
			t.Fatalf("expected HTTP code 200, got %d", res.Code)
		}

		var body metadataCredentials
		err = json.NewDecoder(res.Body).Decode(&body)
		fatalOnErr(t, err)

		expectedCreds := defaultCreds()
		stringsEqual(t, [][2]string{
			[2]string{"Success", body.Code},
			[2]string{*expectedCreds.AccessKeyId, body.AccessKeyID},
			[2]string{"AWS-HMAC", body.Type},
			[2]string{*expectedCreds.SecretAccessKey, body.SecretAccessKey},
			[2]string{*expectedCreds.SessionToken, body.Token},
			[2]string{stsSvc.output.Credentials.Expiration.String(), body.Expiration.String()},
		})

		if *stsSvc.input.RoleArn != config.AliasToARN["db"] {
			t.Fatalf("expected assumed role to be [%s], instead [%s]", config.AliasToARN["db"], *stsSvc.input.RoleArn)
		}
		if *stsSvc.input.Policy != defaultCustomPolicy {
			t.Fatalf("expected custom policy [%s], got [%s]", defaultCustomPolicy, *stsSvc.input.Policy)
		}
	})

	t.Run("should support no selected defaults", func(t *testing.T) {
		config := defaultConfig()
		config.DefaultAlias = ""

		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, ipWithNoLabels)
		fatalOnErr(t, err)

		if res.Code != 404 {
			t.Fatalf("expected HTTP code 404, got %d", res.Code)
		}

		bodyBytes, err := ioutil.ReadAll(res.Body)
		fatalOnErr(t, err)
		if len(bodyBytes) != 0 {
			t.Fatalf("expected empty body, got [%s]", string(bodyBytes))
		}

		if *stsSvc.input.RoleArn != "" {
			t.Fatalf("expected assumed role to be empty, instead [%s]", *stsSvc.input.RoleArn)
		}
		if stsSvc.input.Policy != nil {
			t.Fatalf("expected no policy, got [%s]", *stsSvc.input.Policy)
		}
	})
}
