package proxy

import (
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

	t.Run("should proxy non credentials request", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, "/latest/meta-data/local-hostname", config, stsSvc, containerSvc, defaultIP)
		fatalOnErr(t, err)

		responseCodeIs(t, res, 200)

		bodyBytes, err := ioutil.ReadAll(res.Body)
		fatalOnErr(t, err)
		body := string(bodyBytes)

		if body != defaultProxiedBody {
			t.Fatalf("expected body [%s], got [%s]", defaultProxiedBody, body)
		}
	})
}
