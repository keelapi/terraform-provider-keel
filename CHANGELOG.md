# Changelog

## Unreleased

### Added

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
