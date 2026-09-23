// Package timestamp provides a string attribute type for RFC 3339 timestamps
// that compares values by the instant they name.
//
// Keel normalizes the timestamps it stores and returns them re-serialized:
// "2027-01-01T00:00:00+00:00" comes back as "2027-01-01T00:00:00Z". Comparing
// the strings would make Terraform report "Provider produced inconsistent
// result after apply" for a correct configuration, and plan a replacement
// after import.
package timestamp

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.StringTypable                    = RFC3339Type{}
	_ basetypes.StringValuableWithSemanticEquals = RFC3339{}
)

// RFC3339Type is the attribute type. Values naming the same instant are
// semantically equal, so the framework keeps the prior spelling in state when
// Keel returns another one.
type RFC3339Type struct {
	basetypes.StringType
}

func (t RFC3339Type) String() string {
	return "timestamp.RFC3339Type"
}

func (t RFC3339Type) ValueType(_ context.Context) attr.Value {
	return RFC3339{}
}

func (t RFC3339Type) Equal(o attr.Type) bool {
	other, ok := o.(RFC3339Type)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t RFC3339Type) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return RFC3339{StringValue: in}, nil
}

func (t RFC3339Type) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}
	return RFC3339{StringValue: stringValue}, nil
}

// RFC3339 is a value of RFC3339Type.
type RFC3339 struct {
	basetypes.StringValue
}

// NewNull returns a null timestamp.
func NewNull() RFC3339 {
	return RFC3339{StringValue: basetypes.NewStringNull()}
}

// NewValue returns a known timestamp.
func NewValue(value string) RFC3339 {
	return RFC3339{StringValue: basetypes.NewStringValue(value)}
}

func (v RFC3339) Type(_ context.Context) attr.Type {
	return RFC3339Type{}
}

func (v RFC3339) Equal(o attr.Value) bool {
	other, ok := o.(RFC3339)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals reports whether both values name the same instant.
func (v RFC3339) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	newValue, ok := newValuable.(RFC3339)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			fmt.Sprintf("Expected value type %T but got %T. Please report this issue to the provider developers.", v, newValuable),
		)
		return false, diags
	}
	return SameInstant(v.ValueString(), newValue.ValueString()), diags
}

// SameInstant reports whether a and b are the same string or timestamps naming
// the same instant. A timestamp without a UTC offset is read as UTC, which is
// how Keel interprets one.
func SameInstant(a, b string) bool {
	if a == b {
		return true
	}
	ta, okA := parse(a)
	tb, okB := parse(b)
	return okA && okB && ta.Equal(tb)
}

func parse(value string) (time.Time, bool) {
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, true
	}
	// Fractional seconds are accepted after the seconds field even though the
	// layouts do not spell them out.
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if ts, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

// UseStateForSameInstant returns a plan modifier that keeps the prior state
// value when the configured timestamp names the same instant. Terraform
// accepts a planned value equal to the prior state in place of a non-null
// configuration value, so a re-spelled timestamp, or one Keel spelled
// differently at import, plans no change.
func UseStateForSameInstant() planmodifier.String {
	return useStateForSameInstant{}
}

type useStateForSameInstant struct{}

func (m useStateForSameInstant) Description(_ context.Context) string {
	return "Keeps the prior value when the configured timestamp names the same instant."
}

func (m useStateForSameInstant) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useStateForSameInstant) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() ||
		req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if SameInstant(req.StateValue.ValueString(), req.PlanValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
