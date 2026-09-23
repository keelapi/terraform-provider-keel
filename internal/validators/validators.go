// Package validators holds the attribute validators the provider's schemas
// use. Validators run on configuration only, so a bad value fails at plan time
// instead of as an API error during apply.
package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// OneOf returns a validator that accepts only the given values. Matching is
// case-sensitive: Keel stores these values lowercase and returns them that
// way, so another spelling would not round-trip.
func OneOf(values ...string) validator.String {
	return oneOf{values: values}
}

type oneOf struct {
	values []string
}

func (v oneOf) Description(_ context.Context) string {
	return fmt.Sprintf("value must be one of: %s", quoteAll(v.values))
}

func (v oneOf) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v oneOf) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	for _, allowed := range v.values {
		if value == allowed {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid Attribute Value",
		fmt.Sprintf("Attribute %s %s, got: %q", req.Path, v.Description(ctx), value),
	)
}

// RFC3339 returns a validator that accepts RFC 3339 timestamps with a UTC
// offset, such as "2027-01-01T00:00:00Z".
func RFC3339() validator.String {
	return rfc3339{}
}

type rfc3339 struct{}

func (v rfc3339) Description(_ context.Context) string {
	return `value must be an RFC 3339 timestamp with a UTC offset, such as "2027-01-01T00:00:00Z"`
}

func (v rfc3339) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v rfc3339) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid RFC 3339 Timestamp",
			fmt.Sprintf("Attribute %s %s, got: %q", req.Path, v.Description(ctx), value),
		)
	}
}

func quoteAll(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}
