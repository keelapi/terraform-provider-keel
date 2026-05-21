# Changelog

## Unreleased

### Added

- **`keel_organization_member` resource** backed by `/v1/organizations/{org_id}/members`, including create, read, update, delete, import support, acceptance tests, and example configuration.
- **API key import support** and import acceptance coverage for `keel_api_key`.
- **OPA policy gate example** under `examples/opa-policy-gate`, showing `terraform plan` JSON export, Rego evaluation, and gated apply.
- **v1.0 blocker documentation** for deferred resources under `docs/blockers.md`.
- **HTTP 429 throttle handling with auto-retry.** The HTTP client now detects
  rate-limit throttle responses (HTTP 429), parses the `Retry-After` header
  (with body fallback), and retries automatically. Configurable retry count
  (default 1, hard cap 3). Non-throttle 4xx errors are never retried.
- **`ThrottledError` error type** (`internal/client/errors.go`) exposing
  `RetryAfterSeconds`, `PermitID`, and `ReasonCode`. Returned after retries
  are exhausted on 429.
- **`APIError` error type** (`internal/client/errors.go`) for all non-429
  HTTP errors, replacing the previous untyped `fmt.Errorf`.
- **Shape D reason code constants** (`internal/client/reason_codes.go`):
  11 dot-namespaced reason codes matching keel-api V1.16.0.
- **Permit data source expanded fields**: `reason_code`, `reason_detail`,
  `outcome_detail`, and `message` are now exposed on `keel_permit` permits.
- **Rate limiting documentation** in README.

### Changed

- **`keel_api_key` scope default now matches `/v1/api-keys`: `admin`.** Prior provider behavior defaulted to `client`.
- **`keel_api_key` now exposes `created_by` as a computed field** and accepts optional `project_id` to use project-scoped API key endpoints.
