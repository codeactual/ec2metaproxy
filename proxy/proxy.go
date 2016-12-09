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

	"github.com/aws/aws-sdk-go/service/sts/stsiface"
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
	httpClient    http.RoundTripper
	credsProvider *credentialsProvider
	config        Config
	log           *log.Logger
}

// New creates a Proxy instance using the given configuration.
//
// logger := log.New(os.Stdout, "ec2metaproxy ", log.LstdFlags|log.LUTC)
// config := proxy.Config{ ... }
// p, err := proxy.New(config, sts.New(session.New()), logger)
//
func New(config Config, httpClient http.RoundTripper, stsSvc stsiface.STSAPI, containerSvc containerService, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(new(nopWriter), "", log.LstdFlags)
	}

	var defaultIamRole roleArn
	var err error

	if config.DefaultAlias != "" {
		defaultIamRole, err = newRoleArn(config.AliasToARN[config.DefaultAlias])
		if err != nil {
			return nil, errors.Wrap(err, "Error configuring proxy")
		}
	}

	p := Proxy{
		credsProvider: newCredentialsProvider(stsSvc, containerSvc, defaultIamRole, config.DefaultPolicy),
		httpClient:    httpClient,
		log:           logger,
		config:        config,
	}

	return &p, nil
}

// ServeHTTP can be used to handle "/" requests and will delegate to HandleCredentials
// to produce a response.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := remoteIP(r.RemoteAddr)
	reqID := requestIDFromContext(r.Context())

	p.log.Printf("HandleCredentials (%s): PROXY REQUEST ip [%s] url [%s]", reqID, clientIP, r.URL.String())

	match := credsRegex.FindStringSubmatch(r.URL.Path)
	if match != nil {
		p.HandleCredentials(MetadataURL, match[1], match[2], p.credsProvider, w, r)
		return
	}

	if p.config.Verbose {
		p.log.Printf("ServeHTTP (%s): FORWARD REQUEST ip [%s] path [%s]", reqID, clientIP, r.URL.Path)
	}

	proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", MetadataURL, r.URL.Path), r.Body)

	if err != nil {
		p.log.Printf("ServeHTTP (%s): Error creating proxy http request: %+v", reqID, err)
		http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
		return
	}

	copyHeaders(proxyReq.Header, r.Header)
	resp, err := p.httpClient.RoundTrip(proxyReq)

	if err != nil {
		p.log.Printf("ServeHTTP (%s): Error forwarding request to EC2 metadata service: %+v", reqID, err)
		http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
		return
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			p.log.Printf("ServeHTTP (%s): Error closing respond body: %+v", reqID, closeErr)
		}
	}()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		p.log.Printf("ServeHTTP (%s): Error copying response content from EC2 metadata service: %+v", reqID, err)
	}

	if p.config.Verbose {
		p.log.Printf("ServeHTTP (%s): FORWARD RESPONSE ip [%s] path [%s]", reqID, clientIP, r.URL.Path)
	}
}

// HandleCredentials responds to credentials requests identified in ServeHTTP.
func (p *Proxy) HandleCredentials(baseURL, apiVersion, subpath string, c *credentialsProvider, w http.ResponseWriter, r *http.Request) {
	clientIP := remoteIP(r.RemoteAddr)
	ctx := r.Context()
	reqID := requestIDFromContext(ctx)
	awsURL := baseURL + "/" + apiVersion + "/meta-data/iam/security-credentials/"

	if p.config.Verbose {
		p.log.Printf("HandleCredentials (%s): UPSTREAM REQUEST ip [%s] path [%s]", reqID, clientIP, awsURL)
	}

	awsReq, err := http.NewRequest("GET", awsURL, nil)
	if err != nil {
		p.log.Printf("HandleCredentials (%s): Error creating request [%s]: %+v", reqID, awsURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := p.httpClient.RoundTrip(awsReq)

	if err != nil {
		p.log.Printf("HandleCredentials (%s): Error requesting creds path for API version [%s]: %+v", reqID, apiVersion, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if p.config.Verbose {
		p.log.Printf("HandleCredentials (%s): UPSTREAM RESPONSE ip [%s] path [%s] code [%d]", reqID, clientIP, awsURL, resp.StatusCode)
	}

	err = resp.Body.Close()
	if err != nil {
		p.log.Printf("HandleCredentials (%s): Error closing credentials response body: %+v", reqID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	credentials, err := c.CredentialsForIP(ctx, clientIP)

	if err != nil {
		p.log.Printf("HandleCredentials (%s): Error getting credentials for IP [%s]: %+v", reqID, clientIP, err)
		http.Error(w, "An unexpected error getting container role", http.StatusInternalServerError)
		return
	}

	roleName := credentials.RoleArn.RoleName()
	statusCode := http.StatusOK

	if len(subpath) == 0 {
		_, writeErr := w.Write([]byte(roleName))
		if writeErr != nil {
			p.log.Printf("HandleCredentials (%s): Error writing role name to response: %+v", reqID, writeErr)
		}
	} else if roleName == "" || (!strings.HasPrefix(subpath, roleName) || (len(subpath) > len(roleName) && subpath[len(roleName)-1] != '/')) {
		// An idiosyncrasy of the standard EC2 metadata service:
		// Subpaths of the role name are ignored. So long as the correct role name is provided,
		// it can be followed by a slash and anything after the slash is ignored.
		statusCode = http.StatusNotFound
		w.WriteHeader(statusCode)
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
			p.log.Printf("HandleCredentials (%s): Error marshaling credentials: %+v", reqID, err)
			statusCode = http.StatusInternalServerError
			w.WriteHeader(statusCode)
		} else {
			_, writeErr := w.Write(creds)
			if writeErr != nil {
				p.log.Printf("HandleCredentials (%s): Error writing credentials to response: %+v", reqID, writeErr)
			}
		}
	}

	if p.config.Verbose {
		p.log.Printf("HandleCredentials (%s): PROXY RESPONSE ip [%s] path [%s] role [%s] subpath [%s] code [%d]", reqID, clientIP, awsURL, roleName, subpath, statusCode)
	}
}

// Listen listens on the TCP address defined in the config file.
func (p *Proxy) Listen() error {
	err := http.ListenAndServe(p.config.ListenAddr, nil)
	if err == nil {
		p.log.Printf("Listen: [%s]", p.config.ListenAddr)
	} else {
		return errors.Wrapf(err, "Error listening on address [%s]", p.config.ListenAddr)
	}
	return nil
}
