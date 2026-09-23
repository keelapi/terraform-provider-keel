package timestamp

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSameInstant(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"2027-01-01T00:00:00+00:00", "2027-01-01T00:00:00Z", true},
		{"2027-01-01T00:00:00.000Z", "2027-01-01T00:00:00Z", true},
		{"2027-01-01T02:00:00+02:00", "2027-01-01T00:00:00Z", true},
		// A timestamp without an offset is UTC, as Keel reads it.
		{"2027-01-01T00:00:00+00:00", "2027-01-01T00:00:00", true},
		{"2027-01-01T00:00:00Z", "2027-01-01 00:00:00", true},
		{"2027-01-01T00:00:00Z", "2027-01-01T00:00:01Z", false},
		{"2027-01-01T00:00:00+01:00", "2027-01-01T00:00:00Z", false},
		{"not-a-time", "not-a-time", true},
		{"not-a-time", "2027-01-01T00:00:00Z", false},
	} {
		if got := SameInstant(tc.a, tc.b); got != tc.want {
			t.Errorf("SameInstant(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestStringSemanticEquals(t *testing.T) {
	ctx := context.Background()
	equal, diags := NewValue("2027-01-01T00:00:00+00:00").StringSemanticEquals(ctx, NewValue("2027-01-01T00:00:00Z"))
	if diags.HasError() || !equal {
		t.Fatalf("same instant: equal = %v, diags = %v", equal, diags)
	}
	equal, diags = NewValue("2027-01-01T00:00:00Z").StringSemanticEquals(ctx, NewValue("2027-06-01T00:00:00Z"))
	if diags.HasError() || equal {
		t.Fatalf("different instants: equal = %v, diags = %v", equal, diags)
	}
}

func TestUseStateForSameInstant(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name        string
		state, plan types.String
		want        types.String
	}{
		{
			name:  "re-spelled instant keeps state",
			state: types.StringValue("2027-01-01T00:00:00Z"),
			plan:  types.StringValue("2027-01-01T00:00:00+00:00"),
			want:  types.StringValue("2027-01-01T00:00:00Z"),
		},
		{
			name:  "different instant is planned",
			state: types.StringValue("2027-01-01T00:00:00Z"),
			plan:  types.StringValue("2028-01-01T00:00:00Z"),
			want:  types.StringValue("2028-01-01T00:00:00Z"),
		},
		{
			name:  "create has no state",
			state: types.StringNull(),
			plan:  types.StringValue("2027-01-01T00:00:00+00:00"),
			want:  types.StringValue("2027-01-01T00:00:00+00:00"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			UseStateForSameInstant().PlanModifyString(ctx, planmodifier.StringRequest{
				StateValue:  tc.state,
				ConfigValue: tc.plan,
				PlanValue:   tc.plan,
			}, resp)
			if !resp.PlanValue.Equal(tc.want) {
				t.Errorf("PlanValue = %s, want %s", resp.PlanValue, tc.want)
			}
		})
	}
}
