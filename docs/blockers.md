# v1.0 Deferred Resource API Blockers

> Internal tracker. This file is not part of the Terraform Registry-published provider, resource, or data source reference.

Keel Terraform v1.0 ships only the resources backed by the public keel-api surface available today.

## `keel_workspace`

Deferred to v1.1+.

API gap: the OpenAPI artifact does not define a `/v1/workspaces` CRUD surface. The closest current surface is projects (`/v1/projects` and `/v1/dashboard/projects/{project_id}`), but it does not provide a clean workspace CRUD contract for Terraform.

## `keel_policy_attachment`

Deferred to v1.1+.

API gap: the OpenAPI artifact does not define policy attachment or detach endpoints. Current policy endpoints manage policy objects (`/v1/policies`) and project policy overrides (`/v1/projects/{project_id}/policy`), but neither is an attachment contract.

## `keel_audit_export_config`

Deferred to v1.1+.

API gap: the OpenAPI artifact exposes compliance export job endpoints (`/v1/compliance/exports`) but not a persistent audit export configuration resource with CRUD semantics.
