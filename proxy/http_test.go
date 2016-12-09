package proxy_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/codeactual/ec2metaproxy/proxy"
	"github.com/pkg/errors"
)

type logger struct {
	logger *log.Logger
	events []string
}

func newLogger() *logger {
	l := logger{
		events: []string{},
	}
	l.logger = log.New(&l, "", 0)
	return &l
}

func (l *logger) Write(p []byte) (n int, err error) {
	l.events = append(l.events, string(bytes.TrimSpace(p)))
	return len(p), nil
}

type roundTripperStub struct {
	req *http.Request
	res *http.Response
	err error
}

func (r roundTripperStub) RoundTrip(req *http.Request) (*http.Response, error) {
	r.req = req
	if r.res == nil {
		return nil, errors.New("undefined response in HTTP client stub")
	}
	return r.res, r.err
}

// stubRequest performs a GET against a new Proxy instance using the provided stubs.
//
// The pathSpec argument, ex. "/", is used to create the http.Handle and should match
// a use case like the one in main.go. The pathReq argument is the path to request.
// The separation allows us simulate mismatches for cases like 404.
func stubRequest(pathSpec, pathReq string, config proxy.Config, stsSvc *assumeRoleStub, containerSvc proxy.ContainerService, clientIP string) (*httptest.ResponseRecorder, []string, error) {
	l := newLogger()

	req, err := http.NewRequest("GET", pathReq, nil)
	req.RemoteAddr = clientIP
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to request [%s] from [%s] handler", pathReq, pathSpec)
	}

	httpClient := roundTripperStub{
		req: req,
		res: &http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(defaultProxiedBody)),
			StatusCode: 200,
		},
	}

	p, initErr := proxy.New(config, httpClient, stsSvc, containerSvc, l.logger)
	if initErr != nil {
		return nil, nil, errors.Wrap(initErr, "failed to create proxy")
	}

	recorder := httptest.NewRecorder()
	proxy.RequestID(p).ServeHTTP(recorder, req)

	return recorder, l.events, nil
}
