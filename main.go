package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	defaultMetadataURL = "http://169.254.169.254"
	labelKey           = "ec2metaproxy.RoleAlias"
	policyKey          = "ec2metaproxy.Policy"
)

var (
	credsRegex = regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

	instanceServiceClient = &http.Transport{}
)

type metadataCredentials struct {
	Code            string
	LastUpdated     time.Time
	Type            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

func copyHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}

	for k, v := range src {
		vCopy := make([]string, len(v))
		copy(vCopy, v)
		dst[k] = vCopy
	}
}

func remoteIP(addr string) string {
	index := strings.Index(addr, ":")

	if index < 0 {
		return addr
	}

	return addr[:index]

}

func newGET(path string) *http.Request {
	r, err := http.NewRequest("GET", path, nil)

	if err != nil {
		panic(err)
	}

	return r
}

func handleCredentials(baseURL, apiVersion, subpath string, c *credentialsProvider, w http.ResponseWriter, r *http.Request) {
	resp, err := instanceServiceClient.RoundTrip(newGET(baseURL + "/" + apiVersion + "/meta-data/iam/security-credentials/"))

	if err != nil {
		log.Printf("Error requesting creds path for API version [%s]: %+v", apiVersion, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = resp.Body.Close()
	if err != nil {
		log.Printf("Error closing credentials response body: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	clientIP := remoteIP(r.RemoteAddr)
	credentials, err := c.CredentialsForIP(clientIP)

	if err != nil {
		log.Printf("failed to get credentials for IP [%s]: %+v", clientIP, err)
		http.Error(w, "An unexpected error getting container role", http.StatusInternalServerError)
		return
	}

	roleName := credentials.RoleArn.RoleName()

	if len(subpath) == 0 {
		w.Write([]byte(roleName))
	} else if !strings.HasPrefix(subpath, roleName) || (len(subpath) > len(roleName) && subpath[len(roleName)-1] != '/') {
		// An idiosyncrasy of the standard EC2 metadata service:
		// Subpaths of the role name are ignored. So long as the correct role name is provided,
		// it can be followed by a slash and anything after the slash is ignored.
		w.WriteHeader(http.StatusNotFound)
	} else {
		creds, err := json.Marshal(&metadataCredentials{
			Code:            "Success",
			LastUpdated:     credentials.GeneratedAt,
			Type:            "AWS-HMAC",
			AccessKeyID:     credentials.AccessKey,
			SecretAccessKey: credentials.SecretKey,
			Token:           credentials.Token,
			Expiration:      credentials.Expiration,
		})

		if err != nil {
			log.Printf("Error marshaling credentials: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write(creds)
		}
	}
}

// ProxyConfig describes the JSON config file selected via `-config` flag.
type ProxyConfig struct {
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

var proxyConfig ProxyConfig
var verbose bool

func main() {
	proxyConfig = ProxyConfig{}

	var configFile string
	flag.StringVar(&configFile, "c", "", "Path to JSON config file.")
	flag.BoolVar(&verbose, "v", false, "Print verbose console messages.")
	flag.Parse()

	configBytes, readErr := ioutil.ReadFile(configFile)
	if readErr != nil {
		panic(readErr)
	}
	jsonErr := json.Unmarshal(configBytes, &proxyConfig)
	if jsonErr != nil {
		panic(jsonErr)
	}

	if proxyConfig.ListenAddr == "" {
		panic("Config file must select a server address ('listen', ex. ':18000').")
	}

	defaultIamRole, roleErr := newRoleArn(proxyConfig.AliasToARN[proxyConfig.DefaultAlias])
	if roleErr != nil {
		panic(roleErr)
	}

	platform, platformErr := newDockerContainerService(proxyConfig.DockerHost)
	if platformErr != nil {
		panic(platformErr)
	}

	awsSession := session.New()
	credentials := newCredentialsProvider(awsSession, platform, defaultIamRole, proxyConfig.DefaultPolicy)

	// Proxy non-credentials requests to primary metadata service
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Client [%s] request [%s]", remoteIP(r.RemoteAddr), r.URL.Path)

		match := credsRegex.FindStringSubmatch(r.URL.Path)
		if match != nil {
			handleCredentials(defaultMetadataURL, match[1], match[2], credentials, w, r)
			return
		}

		proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", defaultMetadataURL, r.URL.Path), r.Body)

		if err != nil {
			log.Printf("Error creating proxy http request: %+v", err)
			http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
			return
		}

		copyHeaders(proxyReq.Header, r.Header)
		resp, err := instanceServiceClient.RoundTrip(proxyReq)

		if err != nil {
			log.Printf("Error forwarding request to EC2 metadata service: %+v", err)
			http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("Error copying response content from EC2 metadata service: %+v", err)
		}
	})

	listenErr := http.ListenAndServe(proxyConfig.ListenAddr, nil)
	if listenErr == nil {
		log.Printf("listening on address [%s]\n", proxyConfig.ListenAddr)
	} else {
		log.Fatalf("failed to listen on address [%s]: %+v\n", proxyConfig.ListenAddr, listenErr)
	}
}

func verbosef(format string, a ...interface{}) {
	if verbose {
		log.Printf(format, a...)
	}
}
