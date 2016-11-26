package proxy

import (
	"net/http"
	"strings"
)

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
