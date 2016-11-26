package main

import (
	"log"
	"net/http"
	"os"

	"git.codeactual.com/codeactual/ec2metaproxy/proxy"
)

func main() {
	logger := log.New(os.Stdout, "ec2metaproxy", log.LstdFlags|log.LUTC)

	p, initErr := proxy.New(logger)
	if initErr != nil {
		logger.Fatalf("failed to init proxy: %+v", initErr)
	}

	http.HandleFunc("/", p.HandleUnmatched)

	logger.Fatal(p.Listen())
}
