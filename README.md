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
> **Infrastructure surfaces:** Terraform manages a minimal set of Keel governance resources (API keys and organization membership) and reads permits; it does not manage Keel policies. MCP governance is exposed through `/v1/mcp/*`. Keel should not be described as a generic MCP server or submitted to MCP registries.
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
      version = "~> 1.1"
    }
  }
}

provider "keel" {
  api_key = var.keel_api_key # or set KEEL_API_KEY env var
  # user_token: set KEEL_USER_TOKEN when managing keel_organization_member
}

variable "keel_api_key" {
  type      = string
  sensitive = true
}
```

| Argument     | Environment Variable | Default                   | Description |
|--------------|----------------------|---------------------------|-------------|
| `api_key`    | `KEEL_API_KEY`       | —                         | Keel API key. `keel_api_key` needs admin scope; `keel_permit` accepts admin or client scope. |
| `user_token` | `KEEL_USER_TOKEN`    | —                         | Keel user access token, used only by `keel_organization_member`. |
| `base_url`   | `KEEL_BASE_URL`      | `https://api.keelapi.com` | API base URL. |

Set at least one credential. Each resource uses the credential its Keel routes accept, so one provider block can hold both:

- **API key** (`api_key`): `keel_api_key` and `keel_permit`. Keel scopes both to the project of the API key.
- **User token** (`user_token`): `keel_organization_member`. Keel's organization member routes accept only a signed-in user's credential, never an API key. The user must be an owner or admin of the organization. User tokens are short-lived (a Keel dashboard session token expires after 30 minutes), so pass a fresh one through `KEEL_USER_TOKEN` for each run rather than storing it in configuration.

## Request Lifecycle

When Keel processes AI requests for your project, it follows this high-level flow:

- **Evaluate:** identity, policy, and budget constraints are checked
- **Decide:** a permit decision is issued — allow, deny, review (held for approval), or throttle
- **Execute:** the provider call occurs only if permitted
- **Record:** usage, cost, and governance events are captured

Requests are only executed if explicitly permitted.

## Resources

The provider manages `keel_api_key` and `keel_organization_member` and reads `keel_permit`. Workspaces, policy attachments, and audit export configuration are not available because the Keel API has no matching resource that this provider's credentials can manage. See [docs/blockers.md](docs/blockers.md).

### `keel_api_key`

Create API keys in the project of the provider's API key, which must have admin scope. Keys are immutable — any change triggers replacement. Deletion revokes the key.

```hcl
resource "keel_api_key" "backend" {
  name        = "backend-service"
  description = "Key for the backend service"
  scope       = "client" # admin, client, or approval
  expires_at  = "2027-01-01T00:00:00Z"
}
```

- `scope` defaults to `admin`, matching `POST /v1/api-keys`.
- `project_id` is read-only: it is always the project of the provider's API key.
- `expires_at` is an RFC 3339 timestamp with a UTC offset. Keel may return the same instant spelled differently (for example `Z` for `+00:00`); that is not a change.
- `agent_principal_id` binds the key to an agent principal. It cannot be combined with `scope = "approval"`.
- `scope = "approval"` keys exist only after dual-control approval. Keel refuses them unless the project enforces dual control, and otherwise holds each one for a second approver. Terraform cannot wait for that approval, so the apply fails with the pending change's ID and nothing is stored.
- The secret (`raw_key`) is returned only when the key is created; imported keys have no `raw_key`.

Import with the key ID:

```sh
terraform import keel_api_key.backend <key_id>
```

### `keel_organization_member`

Manage a user's role in a Keel organization. Requires `user_token` (or `KEEL_USER_TOKEN`) for an owner or admin of the organization.

```hcl
resource "keel_organization_member" "reviewer" {
  org_id  = var.org_id
  user_id = var.user_id
  role    = "member" # owner, admin, member, or viewer
}
```

Roles are lowercase. An organization admin can grant only `member` and `viewer`. When the organization enforces dual control, adding or promoting a privileged member is held for approval: the apply fails with the pending change and nothing is stored.

Import with `org_id/user_id`.

## Data Sources

### `keel_permit`

Query recent permits in the project of the provider's API key, newest first.

```hcl
data "keel_permit" "recent_denials" {
  decision = "deny" # allow, deny, review, or throttle
  limit    = 50     # 1-200
}
```

Each permit exposes `decision`, `reason`, `reason_code`, `outcome_kind` (`decision`, `precondition`, or `unclassified`), `message`, `reason_detail` (the decision details as JSON), and `created_at`. `decision = "challenge"` still works as a deprecated alias for `review`.

## OPA Policy Gate

See [examples/opa-policy-gate](examples/opa-policy-gate) for a Terraform plan JSON gate that runs:

```sh
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json
opa eval -d policy.rego -i plan.json 'data.policy.deny'
terraform apply plan.tfplan
```

The example manages an API key and an organization member, so it needs both `KEEL_API_KEY` and `KEEL_USER_TOKEN`.

## Rate Limiting and Errors

When the Keel API answers HTTP 429, the provider waits for the `Retry-After` header (or the `retry_after_seconds` in the error body) and retries the request once. It returns the error without waiting when Keel asks for more than 60 seconds, and when the 429 is Keel's response to repeated authentication failures (`auth_failure_rate_limited`), since retrying with the same credential cannot succeed. Other errors (400, 401, 403, 404, 409) are never retried.

Errors show Keel's error code and message, for example `API error (status 403): approval.authority_configuration_required: ...`.

## Building from Source

```sh
git clone https://github.com/keelapi/terraform-provider-keel.git
cd terraform-provider-keel
make install
```

This builds the provider and installs it to your local Terraform plugin directory as the version in `GNUmakefile` (`make install VERSION=x.y.z` to override).

## Releases

Releases are tag-driven through GitHub Actions: pushing a `vMAJOR.MINOR.PATCH` tag builds the provider for the Terraform Registry platforms, uploads the registry manifest, writes SHA256 checksums, and signs the checksum file with the configured GPG key. The Terraform Registry picks up the GitHub release. See [PUBLISHING.md](PUBLISHING.md) for the release checklist.

## Development

```sh
# Run unit tests. Some drive a local Terraform CLI against a fake Keel API;
# they are skipped when terraform is not on PATH (or TF_ACC_TERRAFORM_PATH).
make test

# Run acceptance tests against a Keel API: KEEL_API_KEY (admin scope) for the
# API key and permit tests; KEEL_USER_TOKEN, KEEL_TEST_ORG_ID and
# KEEL_TEST_USER_ID for the organization member test.
make testacc

# Generate Terraform Registry documentation from templates/ and examples/
make generate-docs
```

`make docs` remains available as an alias for `make generate-docs`.

## License

[MIT](LICENSE)
