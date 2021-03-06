package proxy

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"strings"

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
	// Verbose enables request/response logging to standard out.
	Verbose bool
}

// NewConfigFromFlag constructs a new Config from the JSON file obtained via `-config` CLI flag.
// It also validates the unmarshaled Config fields.
func NewConfigFromFlag() (c Config, err error) {
	var configFile string

	flag.StringVar(&configFile, "c", "", "Path to JSON config file.")
	flag.Parse()

	if configFile == "" {
		return c, errors.New("'-c <file>' flag is required")
	}

	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return c, errors.Wrapf(err, "Error reading config file [%s]", configFile)
	}
	err = json.Unmarshal(configBytes, &c)
	if err != nil {
		return c, errors.Wrapf(err, "Error parsing config file JSON [%s]", configFile)
	}

	if c.ListenAddr == "" {
		return c, errors.New("Config file must select a server address ('listen', ex. ':18000').")
	}
	if len(c.AliasToARN) == 0 {
		return c, errors.New("Config file must include at least one 'aliasToARN' mapping.")
	}
	if c.AliasToARN[c.DefaultAlias] == "" {
		return c, errors.Errorf("Config file selected an default alias [%s] not mapped in `aliasToARN'.", c.DefaultAlias)
	}

	prefix := "unix://"
	if strings.HasPrefix(c.DockerHost, prefix) {
		name := c.DockerHost[7:]
		fi, statErr := os.Stat(name)
		if statErr != nil {
			return c, errors.Wrapf(statErr, "Error during stat of DOCKER_HOST socket [%s]", name)
		}
		if fi.Mode()&os.ModeSocket == 0 {
			return c, errors.Errorf("DOCKER_HOST [%s] is not a socket", name)
		}
	}

	return c, nil
}
