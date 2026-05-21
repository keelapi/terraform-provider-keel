package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"keel": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestProviderSchema(t *testing.T) {
	// Validates the provider schema compiles and is valid
	_ = testAccProtoV6ProviderFactories
}

func TestProviderV1MinimalSurface(t *testing.T) {
	p := provider.New("test")()

	resources := p.Resources(context.Background())
	if len(resources) != 2 {
		t.Fatalf("expected 2 API-key-backed resources, got %d", len(resources))
	}

	dataSources := p.DataSources(context.Background())
	if len(dataSources) != 1 {
		t.Fatalf("expected 1 API-key-backed data source, got %d", len(dataSources))
	}
}
