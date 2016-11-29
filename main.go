package main

import (
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/codeactual/ec2metaproxy/proxy"
)

func main() {
	config, configErr := proxy.NewConfigFromFlag()
	if configErr != nil {
		log.Fatalf("Error reading configuration from flag/file: %+v", configErr)
	}

	logger := log.New(os.Stdout, "ec2metaproxy ", log.LstdFlags|log.LUTC)

	p, initErr := proxy.New(config, sts.New(session.New()), logger)
	if initErr != nil {
		log.Fatalf("Error creating proxy: %+v", initErr)
	}

	http.Handle("/", proxy.RequestID(p))

	logger.Fatal(p.Listen())
}
