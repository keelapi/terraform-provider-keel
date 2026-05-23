# keel-terraform

The official Terraform provider for [Keel](https://keelapi.com) — a permit-first AI governance and execution control plane.

Keel sits between your application and AI providers (OpenAI, Anthropic, Google, xAI, Meta).

Keel is built and published by Keel API, Inc.

> **⚠️ Keel is currently in private beta.** You'll need a Keel account and API key to use this provider.
> [Sign up for early access →](https://dashboard.keelapi.com/signup)

## Surface position

> The OpenAPI specification is the canonical integration contract for all Keel surfaces.
>
> **First-class runtime SDKs:** Python and TypeScript. Release-gated and kept in semantic lockstep with the runtime.
>
> **Infrastructure surfaces:** Terraform is the official policy-as-code surface. MCP governance is exposed through `/v1/mcp/*`. Keel should not be described as a generic MCP server or submitted to MCP registries.
>
> **Generated/reference client:** Go is published as an official generated/reference client for infrastructure teams. It is not a first-class runtime SDK.
>
> **Other languages:** Clients can be generated from the OpenAPI specification. They are not maintained as official Keel SDKs.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.22 (only for building from source)

## Getting Started

### Provider Configuration

```hcl
terraform {
  required_providers {
    keel = {
      source  = "keelapi/keel"
      version = "~> 1.0"
    }
  }
}

provider "keel" {
  api_key = var.keel_api_key # or set KEEL_API_KEY env var
}

variable "keel_api_key" {
  type      = string
  sensitive = true
}
```

| Argument   | Environment Variable | Default                     | Description          |
|------------|---------------------|-----------------------------|----------------------|
| `api_key`  | `KEEL_API_KEY`      | —                           | Your Keel API key    |
| `base_url` | `KEEL_BASE_URL`     | `https://api.keelapi.com`   | API base URL         |

## Request Lifecycle

When Keel processes AI requests for your project, it follows this high-level flow:

- **Evaluate:** identity, policy, and budget constraints are checked
- **Decide:** a permit decision is issued — allow, deny, or constrain
- **Execute:** the provider call occurs only if permitted
- **Record:** usage, cost, and governance events are captured

Requests are only executed if explicitly permitted.

## Resources

v1.0 ships `api_keys` and `organization_member`; `workspaces`, `policy_attachments`, and `audit_export_config` are deferred to v1.1+ pending keel-api API surface definition. See [docs/blockers.md](docs/blockers.md).

### `keel_api_key`

Create API keys for the project associated with the provider API key. The provider API key must have admin scope. Keys are immutable — any change triggers replacement. Deletion revokes the key.

```hcl
resource "keel_api_key" "backend" {
  name        = "backend-service"
  description = "Key for the backend service"
  scope       = "client" # admin, client, or approval
}
```

`scope` defaults to `admin`, matching `/v1/api-keys`. Set `project_id` to use `/v1/projects/{project_id}/api-keys`; omit it to let `/v1/api-keys` derive the project from the provider API key. Import with `key_id` or `project_id/key_id`.

### `keel_organization_member`

Manage a user's role in a Keel organization.

```hcl
resource "keel_organization_member" "reviewer" {
  org_id  = var.org_id
  user_id = var.user_id
  role    = "member"
}
```

Import with `org_id/user_id`.

## Data Sources

### `keel_permit`

Query AI request permits with optional filtering.

```hcl
data "keel_permit" "recent_denials" {
  decision = "deny"
  limit    = 50
}
```

## OPA Policy Gate

See [examples/opa-policy-gate](examples/opa-policy-gate) for a Terraform plan JSON gate that runs:

```sh
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json
opa eval -d policy.rego -i plan.json 'data.policy.deny'
terraform apply plan.tfplan
```

## Rate Limiting and Throttle Handling

The provider's HTTP client automatically handles rate-limit throttling from the Keel API (HTTP 429 responses). When a 429 is received the client parses the `Retry-After` header (falling back to `retry_after_seconds` in the response body) and waits before retrying the request.

By default the client retries once. You can configure up to 3 retries by setting `ThrottleRetries` on the client. After all retries are exhausted, a `ThrottledError` is returned containing `RetryAfterSeconds`, `PermitID`, and `ReasonCode`. Non-throttle errors (403, 404, etc.) are never retried.

```go
c := client.New(baseURL, apiKey)
c.ThrottleRetries = 2 // retry up to 2 times on 429 (hard cap: 3)
```

## Building from Source

```sh
git clone https://github.com/keelapi/terraform-provider-keel.git
cd terraform-provider-keel
make install
```

This builds the provider and installs it to your local Terraform plugin directory.

## Releases

Releases are tag-driven through GitHub Actions. After the Terraform Registry
signing key, GitHub secrets, Registry namespace, and provider listing are
configured, push the next semantic version tag such as `v1.0.1`:

```sh
git tag v1.0.1
git push origin v1.0.1
```

The release workflow builds the provider for the Terraform Registry platforms,
uploads the registry manifest, writes SHA256 checksums, and signs the checksum
file with the configured GPG key. See [PUBLISHING.md](PUBLISHING.md) for the
one-time setup and release checklist.

## Development

```sh
# Run unit tests
make test

# Run acceptance tests (requires KEEL_API_KEY; organization member tests also require
# KEEL_TEST_ORG_ID and KEEL_TEST_USER_ID)
make testacc

# Generate documentation
make docs
```

## License

[MIT](LICENSE)
