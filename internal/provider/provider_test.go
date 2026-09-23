package provider_test

import (
	"context"
	"strings"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
	"github.com/keelapi/terraform-provider-keel/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"keel": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestProviderSchema(t *testing.T) {
	// Validates the provider schema compiles and is valid
	_ = testAccProtoV6ProviderFactories
}

func TestProviderMetadataReportsBuildVersion(t *testing.T) {
	var resp fwprovider.MetadataResponse
	provider.New("1.1.0")().Metadata(context.Background(), fwprovider.MetadataRequest{}, &resp)
	if resp.Version != "1.1.0" {
		t.Fatalf("Version = %q, want the version passed to New", resp.Version)
	}
	if resp.TypeName != "keel" {
		t.Fatalf("TypeName = %q, want keel", resp.TypeName)
	}
}

func TestProviderV1MinimalSurface(t *testing.T) {
	p := provider.New("test")()

	resources := p.Resources(context.Background())
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	dataSources := p.DataSources(context.Background())
	if len(dataSources) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(dataSources))
	}
}

// configureProvider runs the provider's Configure with the given configuration
// values (unset attributes are null) and an environment without Keel settings
// unless env overrides them.
func configureProvider(t *testing.T, attrs map[string]string, env map[string]string) fwprovider.ConfigureResponse {
	t.Helper()
	for _, name := range []string{"KEEL_BASE_URL", "KEEL_API_KEY", "KEEL_USER_TOKEN"} {
		t.Setenv(name, env[name])
	}

	ctx := context.Background()
	p := provider.New("test")()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &schemaResp)
	typ := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name := range typ.AttributeTypes {
		if v, ok := attrs[name]; ok {
			vals[name] = tftypes.NewValue(tftypes.String, v)
		} else {
			vals[name] = tftypes.NewValue(tftypes.String, nil)
		}
	}

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, vals)},
	}, &resp)
	return resp
}

func providerData(t *testing.T, resp fwprovider.ConfigureResponse) *client.ProviderData {
	t.Helper()
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics)
	}
	data, ok := resp.ResourceData.(*client.ProviderData)
	if !ok {
		t.Fatalf("ResourceData = %T, want *client.ProviderData", resp.ResourceData)
	}
	if resp.DataSourceData != resp.ResourceData {
		t.Fatal("data sources and resources must share the provider data")
	}
	return data
}

func TestConfigureRequiresACredential(t *testing.T) {
	resp := configureProvider(t, nil, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error with neither api_key nor user_token")
	}
	detail := resp.Diagnostics[0].Detail()
	for _, want := range []string{"KEEL_API_KEY", "KEEL_USER_TOKEN"} {
		if !strings.Contains(detail, want) {
			t.Errorf("diagnostic %q does not mention %s", detail, want)
		}
	}
}

func TestConfigureAPIKeyOnly(t *testing.T) {
	data := providerData(t, configureProvider(t, map[string]string{"api_key": "keel_sk_admin"}, nil))
	if data.APIKey == nil || data.APIKey.Token != "keel_sk_admin" {
		t.Fatalf("APIKey client = %+v, want token keel_sk_admin", data.APIKey)
	}
	if data.UserToken != nil {
		t.Fatal("UserToken client must be nil without a user token")
	}
	if data.APIKey.BaseURL != "https://api.keelapi.com" {
		t.Errorf("BaseURL = %q, want the default", data.APIKey.BaseURL)
	}
}

func TestConfigureUserTokenOnlyFromEnv(t *testing.T) {
	data := providerData(t, configureProvider(t, nil, map[string]string{
		"KEEL_USER_TOKEN": "dsess_from_env",
		"KEEL_BASE_URL":   "http://127.0.0.1:8080",
	}))
	if data.UserToken == nil || data.UserToken.Token != "dsess_from_env" {
		t.Fatalf("UserToken client = %+v, want token from KEEL_USER_TOKEN", data.UserToken)
	}
	if data.UserToken.BaseURL != "http://127.0.0.1:8080" {
		t.Errorf("BaseURL = %q, want KEEL_BASE_URL", data.UserToken.BaseURL)
	}
	if data.APIKey != nil {
		t.Fatal("APIKey client must be nil without an API key")
	}
}

func TestConfigureBothCredentials(t *testing.T) {
	data := providerData(t, configureProvider(t,
		map[string]string{"user_token": "dsess_config", "base_url": "http://127.0.0.1:9000"},
		map[string]string{"KEEL_API_KEY": "keel_sk_env", "KEEL_USER_TOKEN": "dsess_env"},
	))
	if data.APIKey == nil || data.APIKey.Token != "keel_sk_env" {
		t.Fatalf("APIKey client = %+v, want token from KEEL_API_KEY", data.APIKey)
	}
	if data.UserToken == nil || data.UserToken.Token != "dsess_config" {
		t.Fatalf("UserToken client = %+v, want the configured user_token over the env var", data.UserToken)
	}
	if data.APIKey.BaseURL != "http://127.0.0.1:9000" || data.UserToken.BaseURL != "http://127.0.0.1:9000" {
		t.Errorf("both clients must use the configured base_url")
	}
}
