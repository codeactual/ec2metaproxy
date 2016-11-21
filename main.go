package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMetadataURL = "http://169.254.169.254"
	defaultListenAddr  = ":18000"
)

var (
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

func main() {
	//platform := newDockerContainerService("unix:///var/run/main-docker.sock")
	//awsSession := session.New()
	//credentials := newCredentialsProvider(awsSession, platform, *defaultIamRole, *defaultIamPolicy)
	//credsRegex := regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

	// Proxy non-credentials requests to primary metadata service
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// match := credsRegex.FindStringSubmatch(r.URL.Path)
		// if match != nil {
		// handleCredentials(*metadataURL, match[1], match[2], credentials, w, r)
		// return
		// }

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

	http.ListenAndServe(defaultListenAddr, nil)
}
