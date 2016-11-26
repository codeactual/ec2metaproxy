package proxy

// This file was copied from https://github.com/zenazn/goji/blob/master/web/middleware/request_id.go
// at commit 8459820, decoupled, and then updated to use 1.7+ contexts using the tutorial
// at https://joeshaw.org/revisiting-context-and-http-handler-for-go-17/.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

const requestIDKey = "ec2metaproxyReqID"

var reqIDPrefix string
var reqID uint64

/*
A quick note on the statistics here: we're trying to calculate the chance that
two randomly generated base62 prefixes will collide. We use the formula from
http://en.wikipedia.org/wiki/Birthday_problem

P[m, n] \approx 1 - e^{-m^2/2n}

We ballpark an upper bound for $m$ by imagining (for whatever reason) a server
that restarts every second over 10 years, for $m = 86400 * 365 * 10 = 315360000$

For a $k$ character base-62 identifier, we have $n(k) = 62^k$

Plugging this in, we find $P[m, n(10)] \approx 5.75%$, which is good enough for
our purposes, and is surely more than anyone would ever need in practice -- a
process that is rebooted a handful of times a day for a hundred years has less
than a millionth of a percent chance of generating two colliding IDs.
*/

func init() {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	var buf [12]byte
	var b64 string
	for len(b64) < 10 {
		_, readErr := rand.Read(buf[:])
		if readErr != nil {
			panic(fmt.Sprintf("Error creating random prefix for ec2metaproxy request IDs: %+v", readErr))
		}
		b64 = base64.StdEncoding.EncodeToString(buf[:])
		b64 = strings.NewReplacer("+", "", "/", "").Replace(b64)
	}
	reqIDPrefix = fmt.Sprintf("%s/%s", hostname, b64[0:10])
}

func newContextWithRequestID(ctx context.Context, req *http.Request) context.Context {
	id := req.Header.Get("x-ec2metaproxy-id")
	if id == "" {
		id = fmt.Sprintf("%s-%06d", reqIDPrefix, atomic.AddUint64(&reqID, 1))
	}
	return context.WithValue(ctx, requestIDKey, id)
}

func requestIDFromContext(ctx context.Context) string {
	return ctx.Value(requestIDKey).(string)
}

// RequestID is a middleware that injects a request ID into the context of each
// request. A request ID is a string of the form "host.example.com/random-0001",
// where "random" is a base62 random string that uniquely identifies this go
// process, and where the last number is an atomically incremented request
// counter.
func RequestID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := newContextWithRequestID(r.Context(), r)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
