# keel-terraform

The official Terraform provider for [Keel](https://keelapi.com) — a permit-first AI governance and execution control plane.

Keel sits between your application and AI providers (OpenAI, Anthropic, Google, xAI, Meta).

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

### `keel_project`

Manage an AI project with default provider settings, budget limits, and rate limits.

```hcl
resource "keel_project" "production" {
  name        = "production-api"
  description = "Production AI services"

  settings {
    default_provider = "openai"
    default_model    = "gpt-4o"
    budget_limit_usd = 1000.00
    rate_limit_rpm   = 600
  }
}
```

### `keel_api_key`

Create API keys scoped to a project. Keys are immutable — any change triggers replacement.

```hcl
resource "keel_api_key" "backend" {
  project_id = keel_project.production.id
  name       = "backend-service"
  scope      = "project"
}
```

### `keel_policy_rule`

Define governance policies with conditions and actions (`allow`, `deny`, or `constrain`).

```hcl
resource "keel_policy_rule" "cost_cap" {
  name     = "daily-cost-cap"
  priority = 10

  condition {
    field    = "resource.attributes.estimated_cost_usd"
    operator = "greater_than"
    value    = "50.00"
  }

  action  = "deny"
  reason  = "Single request cost exceeds $50 daily cap"
  enabled = true
}
```

### `keel_budget_envelope`

Set budget controls per project with alert thresholds and hard caps.

```hcl
resource "keel_budget_envelope" "q2_budget" {
  project_id   = keel_project.production.id
  name         = "Q2-2026-AI-Budget"
  amount_usd   = 10000.00
  period       = "monthly"
  alert_at_pct = [50, 75, 90]
  hard_cap     = true
}
```

### `keel_routing_config`

Configure multi-provider AI model routing with weights and priorities.

```hcl
resource "keel_routing_config" "multi_provider" {
  project_id = keel_project.production.id

  route {
    provider = "openai"
    model    = "gpt-4o"
    weight   = 70
    priority = 1
  }

  route {
    provider = "anthropic"
    model    = "claude-3-5-sonnet"
    weight   = 30
    priority = 2
  }

  fallback_provider = "openai"
  fallback_model    = "gpt-4o-mini"
}
```

### `keel_provider_key`

Bring your own API keys for upstream AI providers (`openai`, `anthropic`, `google`, `xai`, `meta`).

```hcl
resource "keel_provider_key" "openai_prod" {
  project_id = keel_project.production.id
  provider   = "openai"
  key_value  = var.openai_api_key
  enabled    = true
}
```

## Data Sources

### `keel_project`

Look up an existing project by ID.

```hcl
data "keel_project" "existing" {
  id = "proj_abc123"
}
```

### `keel_permit`

Query AI request permits with optional filtering.

```hcl
data "keel_permit" "recent_denials" {
  decision = "deny"
  limit    = 50
}
```

### `keel_usage_summary`

Get usage metrics (cost, requests, tokens) for a project over a date range.

```hcl
data "keel_usage_summary" "april" {
  project_id = keel_project.production.id
  start_date = "2026-04-01"
  end_date   = "2026-04-30"
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
