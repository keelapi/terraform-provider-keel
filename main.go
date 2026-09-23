package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

// version is set at build time: goreleaser passes the release version and
// `make build` passes $(VERSION), both with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/keelapi/keel",
	})
	if err != nil {
		log.Fatal(err)
	}
}
