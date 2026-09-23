package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

// Helpers for calling resource methods directly, the way the framework does.

func resourceSchema(t *testing.T, r resource.Resource) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// objectValue builds a value of the schema's object type; attributes not in
// attrs are null.
func objectValue(t *testing.T, s schema.Schema, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	typ, ok := s.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}
	for name := range attrs {
		if _, ok := typ.AttributeTypes[name]; !ok {
			t.Fatalf("schema has no attribute %q", name)
		}
	}
	vals := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		if v, ok := attrs[name]; ok {
			vals[name] = v
			continue
		}
		vals[name] = tftypes.NewValue(attrType, nil)
	}
	return tftypes.NewValue(typ, vals)
}

func tfString(v string) tftypes.Value {
	return tftypes.NewValue(tftypes.String, v)
}

func testState(t *testing.T, s schema.Schema, attrs map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: s, Raw: objectValue(t, s, attrs)}
}

func testPlan(t *testing.T, s schema.Schema, attrs map[string]tftypes.Value) tfsdk.Plan {
	t.Helper()
	return tfsdk.Plan{Schema: s, Raw: objectValue(t, s, attrs)}
}

func testConfig(t *testing.T, s schema.Schema, attrs map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	return tfsdk.Config{Schema: s, Raw: objectValue(t, s, attrs)}
}

func emptyState(t *testing.T, s schema.Schema) tfsdk.State {
	t.Helper()
	typ := s.Type().TerraformType(context.Background())
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(typ, nil)}
}

// stateString reads a string attribute; ok is false when it is null.
func stateString(t *testing.T, st tfsdk.State, name string) (string, bool) {
	t.Helper()
	var v *string
	if diags := st.GetAttribute(context.Background(), path.Root(name), &v); diags.HasError() {
		t.Fatalf("reading %q from state: %v", name, diags)
	}
	if v == nil {
		return "", false
	}
	return *v, true
}

func configure(t *testing.T, r resource.ResourceWithConfigure, data *client.ProviderData) diag.Diagnostics {
	t.Helper()
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: data}, &resp)
	return resp.Diagnostics
}

// diagText joins every diagnostic's summary and detail.
func diagText(diags diag.Diagnostics) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Summary())
		b.WriteString(": ")
		b.WriteString(d.Detail())
		b.WriteString("\n")
	}
	return b.String()
}
