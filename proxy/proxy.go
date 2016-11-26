package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
)

var credsRegex = regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

type metadataCredentials struct {
	Code            string
	LastUpdated     time.Time
	Type            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

// Proxy provides HTTP handlers for responding to container requests and mediates requests
// to the real upstream metadata service. Its mediation duties also include mapping
// containers to the roles identified in their (docker) metadata, caching of
// container/credential information, and (optional) operational logging.
type Proxy struct {
	httpClient    *http.Transport
	credsProvider *credentialsProvider
	config        Config
	log           *log.Logger
}

// New creates a Proxy instance using the given configuration.
func New(config Config, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(new(nopWriter), "", log.LstdFlags)
	}

	defaultIamRole, err := newRoleArn(config.AliasToARN[config.DefaultAlias])
	if err != nil {
		return nil, errors.Wrap(err, "Error configuring proxy")
	}

	platform, err := newDockerContainerService(config.DockerHost, config.AliasToARN, logger)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating proxy's container service")
	}

	p := Proxy{
		credsProvider: newCredentialsProvider(session.New(), platform, defaultIamRole, config.DefaultPolicy),
		httpClient:    &http.Transport{},
		log:           logger,
		config:        config,
	}

	return &p, nil
}

// HandleUnmatched can be used to handle "/" requests and will delegate to HandleCredentials
// to produce a response.
func (p *Proxy) HandleUnmatched(w http.ResponseWriter, r *http.Request) {
	match := credsRegex.FindStringSubmatch(r.URL.Path)
	if match != nil {
		p.HandleCredentials(MetadataURL, match[1], match[2], p.credsProvider, w, r)
		return
	}

	proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", MetadataURL, r.URL.Path), r.Body)

	if err != nil {
		p.log.Printf("HandleUnmatched: Error creating proxy http request: %+v", err)
		http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
		return
	}

	copyHeaders(proxyReq.Header, r.Header)
	resp, err := p.httpClient.RoundTrip(proxyReq)

	if err != nil {
		p.log.Printf("HandleUnmatched: Error forwarding request to EC2 metadata service: %+v", err)
		http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
		return
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			p.log.Printf("HandleUnmatched: Error closing respond body: %+v", closeErr)
		}
	}()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		p.log.Printf("HandleUnmatched: Error copying response content from EC2 metadata service: %+v", err)
	}
}

// HandleCredentials responds to credentials requests identified in HandleUnmatched.
func (p *Proxy) HandleCredentials(baseURL, apiVersion, subpath string, c *credentialsProvider, w http.ResponseWriter, r *http.Request) {
	awsURL := baseURL + "/" + apiVersion + "/meta-data/iam/security-credentials/"

	awsReq, err := http.NewRequest("GET", awsURL, nil)
	if err != nil {
		p.log.Printf("HandleCredentials: Error creating request [%s]: %+v", awsURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := p.httpClient.RoundTrip(awsReq)

	if err != nil {
		p.log.Printf("HandleCredentials: Error requesting creds path for API version [%s]: %+v", apiVersion, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = resp.Body.Close()
	if err != nil {
		p.log.Printf("HandleCredentials: Error closing credentials response body: %+v", err)
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
		p.log.Printf("HandleCredentials: Error getting credentials for IP [%s]: %+v", clientIP, err)
		http.Error(w, "An unexpected error getting container role", http.StatusInternalServerError)
		return
	}

	roleName := credentials.RoleArn.RoleName()

	if len(subpath) == 0 {
		_, writeErr := w.Write([]byte(roleName))
		if writeErr != nil {
			p.log.Printf("HandleCredentials: Error writing role name to response: %+v", writeErr)
		}
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
			p.log.Printf("HandleCredentials: Error marshaling credentials: %+v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			_, writeErr := w.Write(creds)
			if writeErr != nil {
				p.log.Printf("HandleCredentials: Error writing credentials to response: %+v", writeErr)
			}
		}
	}
}

// Listen listens on the TCP address defined in the config file.
func (p *Proxy) Listen() error {
	err := http.ListenAndServe(p.config.ListenAddr, nil)
	if err == nil {
		p.log.Printf("listening on address [%s]", p.config.ListenAddr)
	} else {
		return errors.Wrapf(err, "Error listening on address [%s]", p.config.ListenAddr)
	}
	return nil
}
