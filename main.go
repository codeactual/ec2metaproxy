package main

import (
	"log"
	"net/http"
	"os"

	"git.codeactual.com/codeactual/ec2metaproxy/proxy"
)

func main() {
	config, configErr := proxy.NewConfigFromFlag()
	if configErr != nil {
		log.Fatalf("Error reading configuration from flag/file: %+v", configErr)
	}

	logger := log.New(os.Stdout, "ec2metaproxy ", log.LstdFlags|log.LUTC)

	p, initErr := proxy.New(config, logger)
	if initErr != nil {
		log.Fatalf("Error creating proxy: %+v", initErr)
	}

	http.HandleFunc("/", p.HandleUnmatched)

	logger.Fatal(p.Listen())
}
