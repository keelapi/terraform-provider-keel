package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New("dev"), providerserver.ServeOpts{
		Address: "registry.terraform.io/keelapi/keel",
	})
	if err != nil {
		log.Fatal(err)
	}
}
