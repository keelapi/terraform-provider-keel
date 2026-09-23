package datasources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

func configureDataSource(t *testing.T, d datasource.DataSourceWithConfigure, data *client.ProviderData) datasource.ConfigureResponse {
	t.Helper()
	var resp datasource.ConfigureResponse
	var providerData any
	if data != nil {
		providerData = data
	}
	d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: providerData}, &resp)
	return resp
}

func TestPermitConfigureRequiresAPIKey(t *testing.T) {
	d := &permitDataSource{}
	resp := configureDataSource(t, d, &client.ProviderData{UserToken: client.New("http://127.0.0.1:1", "dsess_user")})
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error without an API key")
	}
	if detail := resp.Diagnostics[0].Detail(); !strings.Contains(detail, "KEEL_API_KEY") {
		t.Fatalf("diagnostic %q does not name KEEL_API_KEY", detail)
	}
	if d.client != nil {
		t.Fatal("data source must not fall back to the user token client")
	}
}

func TestPermitConfigureUsesAPIKey(t *testing.T) {
	d := &permitDataSource{}
	apiClient := client.New("http://127.0.0.1:1", "keel_sk_client")
	resp := configureDataSource(t, d, &client.ProviderData{
		APIKey:    apiClient,
		UserToken: client.New("http://127.0.0.1:1", "dsess_user"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if d.client != apiClient {
		t.Fatal("keel_permit must use the API key client")
	}
	if resp := configureDataSource(t, &permitDataSource{}, nil); resp.Diagnostics.HasError() {
		t.Fatalf("unconfigured provider must not be an error: %v", resp.Diagnostics)
	}
}
