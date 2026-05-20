# keel-terraform

The official Terraform provider for [Keel](https://keelapi.com) — a permit-first AI governance and execution control plane.

Keel sits between your application and AI providers (OpenAI, Anthropic, Google, xAI, Meta).

Keel is built and published by Keel API, Inc.

> **⚠️ Keel is currently in private beta.** You'll need a Keel account and API key to use this provider.
> [Sign up for early access →](https://dashboard.keelapi.com/signup)

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
      version = "~> 0.1"
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

This provider is currently API-key-only. It only exposes endpoints that can be managed with a Keel API key.

Dashboard/user-authenticated surfaces such as projects, policies, budget envelopes, routing policies, and provider keys are intentionally not registered in this mode.

### `keel_api_key`

Create API keys for the project associated with the provider API key. The provider API key must have admin scope. Keys are immutable — any change triggers replacement. Deletion revokes the key.

```hcl
resource "keel_api_key" "backend" {
  name        = "backend-service"
  description = "Key for the backend service"
  scope       = "client" # admin, client, or approval
}
```

## Data Sources

### `keel_permit`

Query AI request permits with optional filtering.

```hcl
data "keel_permit" "recent_denials" {
  decision = "deny"
  limit    = 50
}
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

## Development

```sh
# Run unit tests
make test

# Run acceptance tests (requires KEEL_API_KEY)
make testacc

# Generate documentation
make docs
```

## License

[MIT](LICENSE)
