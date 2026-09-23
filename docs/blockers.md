# Deferred Resource API Blockers

> Internal tracker. This file is not part of the Terraform Registry-published provider, resource, or data source reference.

Keel Terraform ships only the resources backed by public Keel API routes that the provider's credentials can use. Reviewed against the Keel API contract on 2026-09-22.

Most Keel configuration routes authenticate a signed-in user (a user access token or dashboard session), not an API key. The provider holds an API key for `/v1/api-keys` and `/v1/permits`, and an optional short-lived user token (`user_token`) used only by `keel_organization_member`.

## `keel_workspace`

Deferred.

API gap: the OpenAPI artifact does not define a `/v1/workspaces` CRUD surface. The closest current surface is projects: `POST`/`GET /v1/projects` (user credential only, with no public read, update or delete by ID) and `PATCH`/`DELETE /v1/dashboard/projects/{project_id}` (dashboard session). Neither is a workspace CRUD contract for Terraform.

## `keel_policy_attachment`

Deferred.

API gap: the OpenAPI artifact does not define policy attachment or detach endpoints. Current policy endpoints manage policy objects (`/v1/policies`) and project policy overrides (`/v1/projects/{project_id}/policy`), but neither is an attachment contract. Both accept only a user credential, and policy writes can be held for dual-control approval, so even a plain `keel_policy` resource is not reachable with an API key.

## `keel_audit_export_config`

Deferred.

API gap: the OpenAPI artifact exposes compliance export job endpoints (`/v1/compliance/exports`) but not a persistent audit export configuration resource with CRUD semantics. Integration configurations (for example SIEM or tracing exports) exist only under `/v1/dashboard/projects/{project_id}/integrations/*`, which need a dashboard session.

## Candidate: webhook subscriptions

Not blocked. `/v1/webhooks` (create, list, get, update, delete, plus `/test` and `/deliveries`) accepts an admin- or client-scope API key, which makes it the only persistent event-export configuration an API-key provider can manage today. The create body is `WebhookSubscriptionCreate` (`callback_url`, `secret` of at least 16 characters, `event_types`, `categories`, `name`, `description`, `is_active`); creation may be held for approval (`OperationalPendingChangeResponse`). A `keel_webhook_subscription` resource is a candidate for a future release.
