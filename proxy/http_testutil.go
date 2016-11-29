package proxy

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
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

// stubRequest performs a GET against a new Proxy instance using the provided stubs.
//
// The pathSpec argument, ex. "/", is used to create the http.Handle and should match
// a use case like the one in main.go. The pathReq argument is the path to request.
// The separation allows us simulate mismatches for cases like 404.
func stubRequest(pathSpec, pathReq string, config Config, stsSvc *assumeRoleStub, containerSvc containerService) (*httptest.ResponseRecorder, []string, error) {
	l := newLogger()

	p, initErr := New(config, stsSvc, containerSvc, l.logger)
	if initErr != nil {
		return nil, nil, errors.Wrap(initErr, "failed to create proxy")
	}

	http.Handle(pathSpec, RequestID(p))

	req, err := http.NewRequest("GET", pathReq, nil)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to request [%s] from [%s] handler", pathReq, pathSpec)
	}

	recorder := httptest.NewRecorder()
	p.ServeHTTP(recorder, req)

	return recorder, l.events, nil
}

type assumeRoleFn func(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)

// assumeRoleStub can be passed to proxy.New to avoid real AWS requests, control AssumeRole behavior,
// and record input.
type assumeRoleStub struct {
	stsiface.STSAPI
	fn    assumeRoleFn
	input *sts.AssumeRoleInput
}

// AssumeROle records input and returns configured output/error.
func (s *assumeRoleStub) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	s.input = input
	return s.fn(input)
}

func newAssumeRoleStubReturns(output *sts.AssumeRoleOutput, err error) *assumeRoleStub {
	return &assumeRoleStub{
		fn: func(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
			return output, err
		},
	}
}
