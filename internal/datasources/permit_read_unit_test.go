package datasources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/keelapi/terraform-provider-keel/internal/client"
)

// testdata/permits_list.json is a GET /v1/permits response from the Keel API
// (PermitAuditListResponse, trimmed to a subset of each item's fields).

func readPermits(t *testing.T, decision, limit tftypes.Value) (datasource.ReadResponse, string) {
	t.Helper()
	fixture, err := os.ReadFile("testdata/permits_list.json")
	if err != nil {
		t.Fatal(err)
	}
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/permits" {
			t.Errorf("path = %q, want /v1/permits", r.URL.Path)
		}
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	d := &permitDataSource{client: client.New(srv.URL, "keel_sk_client")}
	var schemaResp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)
	typ := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, map[string]tftypes.Value{
		"decision": decision,
		"limit":    limit,
		"permits":  tftypes.NewValue(typ.AttributeTypes["permits"], nil),
	})}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, nil)}}
	d.Read(context.Background(), datasource.ReadRequest{Config: config}, &resp)
	return resp, rawQuery
}

func TestPermitReadMapsDecisionDetails(t *testing.T) {
	resp, query := readPermits(t, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.Number, 10))
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}
	if query != "limit=10" {
		t.Errorf("query = %q, want limit=10", query)
	}

	var permits []permitModel
	if diags := resp.State.GetAttribute(context.Background(), path.Root("permits"), &permits); diags.HasError() {
		t.Fatal(diags)
	}
	if len(permits) != 5 {
		t.Fatalf("got %d permits, want 5", len(permits))
	}

	// Explicit decision details: code and reason come from them.
	denied := permits[0]
	if denied.Decision.ValueString() != "deny" || denied.ReasonCode.ValueString() != "policy.rule_denied" {
		t.Errorf("permit 0: decision %s, reason_code %s", denied.Decision, denied.ReasonCode)
	}
	if got := denied.Message.ValueString(); got != "Blocked by the production model rule." {
		t.Errorf("permit 0: message = %q", got)
	}
	if denied.OutcomeKind.ValueString() != "decision" {
		t.Errorf("permit 0: outcome_kind = %s, want decision", denied.OutcomeKind)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(denied.ReasonDetail.ValueString()), &details); err != nil {
		t.Fatalf("permit 0: reason_detail is not JSON: %v", err)
	}
	if details["code"] != "policy.rule_denied" || details["policy_id"] != "starter_preset" {
		t.Errorf("permit 0: reason_detail = %v", details)
	}
	if !denied.OutcomeDetail.IsNull() {
		t.Errorf("permit 0: outcome_detail = %s, want null (deprecated)", denied.OutcomeDetail)
	}

	// No decision details: reason_code falls back to reason; no message.
	throttled := permits[1]
	if throttled.Decision.ValueString() != "throttle" || throttled.ReasonCode.ValueString() != "budget.rate_limit_throttled" {
		t.Errorf("permit 1: decision %s, reason_code %s", throttled.Decision, throttled.ReasonCode)
	}
	if !throttled.Message.IsNull() || !throttled.ReasonDetail.IsNull() {
		t.Errorf("permit 1: message %s, reason_detail %s, want null", throttled.Message, throttled.ReasonDetail)
	}

	if permits[2].Decision.ValueString() != "review" {
		t.Errorf("permit 2: decision = %s, want review", permits[2].Decision)
	}
	if permits[4].OutcomeKind.ValueString() != "unclassified" {
		t.Errorf("permit 4: outcome_kind = %s, want unclassified", permits[4].OutcomeKind)
	}
}

func TestPermitReadSendsReviewForDeprecatedChallenge(t *testing.T) {
	for decision, want := range map[string]string{
		"challenge": "decision=review",
		"review":    "decision=review",
		"throttle":  "decision=throttle",
	} {
		resp, query := readPermits(t, tftypes.NewValue(tftypes.String, decision), tftypes.NewValue(tftypes.Number, nil))
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: Read returned errors: %v", decision, resp.Diagnostics)
		}
		if query != want {
			t.Errorf("decision %q sent query %q, want %q", decision, query, want)
		}
		var got *string
		resp.State.GetAttribute(context.Background(), path.Root("decision"), &got)
		if got == nil || *got != decision {
			t.Errorf("state decision = %v, want the configured %q", got, decision)
		}
	}
}
