package proxy_test

import (
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/codeactual/ec2metaproxy/proxy"
)

func ExampleNew() {
	config, configErr := proxy.NewConfigFromFlag()
	if configErr != nil {
		log.Fatalf("Error reading configuration from flag/file: %+v", configErr)
	}

	logger := log.New(os.Stdout, "ec2metaproxy ", log.LstdFlags|log.LUTC)

	containerSvc, dockerErr := proxy.NewDockerContainerService(config, logger)
	if dockerErr != nil {
		log.Fatalf("Error creating Docker service: %+v", dockerErr)
	}

	p, initErr := proxy.New(config, &http.Transport{}, sts.New(session.New()), containerSvc, logger)
	if initErr != nil {
		log.Fatalf("Error creating proxy: %+v", initErr)
	}

	http.Handle("/", proxy.RequestID(p))
	logger.Fatal(p.Listen())
}
