package main

import (
	"context"
	"flag"
	"log"

	"github.com/adduc/terraform-provider-docker/internal"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary.
	version string = "dev"

	// goreleaser can pass other information to the main package, such as the specific commit
	// https://goreleaser.com/cookbooks/using-main.version/
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// The registry drops the "terraform-provider-" prefix from the
		// repository name, so this is published as adduc/docker.
		Address: "registry.terraform.io/adduc/docker",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), internal.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
