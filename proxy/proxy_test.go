package proxy

import (
	"encoding/json"
	"testing"
)

func TestRoleLabel(t *testing.T) {
	t.Run("valid alias", func(t *testing.T) {
		config := defaultConfig()
		stsSvc := defaultStsSvcStub()
		containerSvc := defaultContainerSvcStub()

		res, _, err := stubRequest(defaultPathSpec, defaultPathReq, config, stsSvc, containerSvc, defaultIP())
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
	})

	t.Run("empty alias", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("unknown alias", func(t *testing.T) {
		t.Skip("TODO")
	})
}
