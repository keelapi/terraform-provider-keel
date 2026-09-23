package validators

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validateString(v validator.String, value types.String) *validator.StringResponse {
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("attr"),
		ConfigValue: value,
	}, resp)
	return resp
}

func TestOneOf(t *testing.T) {
	roles := OneOf("owner", "admin", "member", "viewer")
	for _, tc := range []struct {
		value   types.String
		wantErr bool
	}{
		{types.StringValue("member"), false},
		{types.StringValue("viewer"), false},
		{types.StringValue("Member"), true}, // Keel would store "member": not the configured value
		{types.StringValue(" member"), true},
		{types.StringValue("superuser"), true},
		{types.StringNull(), false},
		{types.StringUnknown(), false},
	} {
		resp := validateString(roles, tc.value)
		if resp.Diagnostics.HasError() != tc.wantErr {
			t.Errorf("OneOf(%s): HasError = %v, want %v (%v)", tc.value, resp.Diagnostics.HasError(), tc.wantErr, resp.Diagnostics)
		}
	}

	resp := validateString(roles, types.StringValue("Member"))
	want := `Attribute attr value must be one of: "owner", "admin", "member", "viewer", got: "Member"`
	if got := resp.Diagnostics[0].Detail(); got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
}

func TestRFC3339(t *testing.T) {
	v := RFC3339()
	for _, tc := range []struct {
		value   string
		wantErr bool
	}{
		{"2027-01-01T00:00:00Z", false},
		{"2027-01-01T00:00:00+00:00", false},
		{"2027-01-01T02:00:00+02:00", false},
		{"2027-01-01T00:00:00.123Z", false},
		{"2027-01-01T00:00:00", true}, // no UTC offset
		{"2027-01-01", true},
		{"tomorrow", true},
	} {
		resp := validateString(v, types.StringValue(tc.value))
		if resp.Diagnostics.HasError() != tc.wantErr {
			t.Errorf("RFC3339(%q): HasError = %v, want %v", tc.value, resp.Diagnostics.HasError(), tc.wantErr)
		}
	}
	if resp := validateString(v, types.StringNull()); resp.Diagnostics.HasError() {
		t.Errorf("null must be accepted: %v", resp.Diagnostics)
	}
}

func TestOneOfWithDeprecatedAliases(t *testing.T) {
	decisions := OneOfWithDeprecatedAliases(
		[]string{"allow", "deny", "review", "throttle"},
		map[string]string{"challenge": "review"},
	)

	resp := validateString(decisions, types.StringValue("review"))
	if len(resp.Diagnostics) != 0 {
		t.Fatalf("review: unexpected diagnostics %v", resp.Diagnostics)
	}

	resp = validateString(decisions, types.StringValue("challenge"))
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("challenge: want exactly one warning, got %v", resp.Diagnostics)
	}
	want := `Attribute decision value "challenge" is deprecated; use "review". The provider sends "review" for it.`
	want = strings.Replace(want, "decision", "attr", 1)
	if got := resp.Diagnostics[0].Detail(); got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}

	resp = validateString(decisions, types.StringValue("allowed"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("allowed: expected an error")
	}
}

func TestInt64Between(t *testing.T) {
	limit := Int64Between(1, 200)
	for _, tc := range []struct {
		value   types.Int64
		wantErr bool
	}{
		{types.Int64Value(1), false},
		{types.Int64Value(200), false},
		{types.Int64Value(0), true},
		{types.Int64Value(500), true},
		{types.Int64Null(), false},
		{types.Int64Unknown(), false},
	} {
		resp := &validator.Int64Response{}
		limit.ValidateInt64(context.Background(), validator.Int64Request{Path: path.Root("limit"), ConfigValue: tc.value}, resp)
		if resp.Diagnostics.HasError() != tc.wantErr {
			t.Errorf("Int64Between(%s): HasError = %v, want %v", tc.value, resp.Diagnostics.HasError(), tc.wantErr)
		}
	}
}
