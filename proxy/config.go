package proxy

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/pkg/errors"
)

// Config describes the JSON config file selected via `-config` flag.
type Config struct {
	// AliasToARN maps human-friendly names to IAM ARNs.
	AliasToARN map[string]string `json:"aliasToARN"`
	// DefaultAlias is a AliasToARN key to select the default role for containers whose
	// metadata does not specify one.
	DefaultAlias string `json:"defaultAlias"`
	// DefaultPolicy restricts the effective role's permissions to the intersection of
	// the role's policy and this JSON policy.
	DefaultPolicy string `json:"defaultPolicy"`
	// DockerHost is a valid DOCKER_HOST string.
	DockerHost string `json:"dockerHost"`
	// ListenAddr is a TCP network address.
	ListenAddr string `json:"listen"`
}

func NewConfigFromFlag() (c Config, err error) {
	var configFile string

	flag.StringVar(&configFile, "c", "", "Path to JSON config file.")
	flag.Parse()

	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return c, errors.Wrapf(err, "failed to read config file [%s]", configFile)
	}
	err = json.Unmarshal(configBytes, &c)
	if err != nil {
		return c, errors.Wrapf(err, "failed to parse config file JSON [%s]", configFile)
	}

	if c.ListenAddr == "" {
		return c, errors.New("Config file must select a server address ('listen', ex. ':18000').")
	}

	return c, nil
}
