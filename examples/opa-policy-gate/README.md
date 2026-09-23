# OPA Policy Gate

This example gates a Keel Terraform plan with Open Policy Agent before apply:

```sh
export KEEL_API_KEY="..."    # admin-scope Keel API key
export KEEL_USER_TOKEN="..." # Keel user token of an organization owner or admin
terraform init
./gate.sh
```

The configuration manages a `keel_api_key`, a `keel_organization_member` and
reads `keel_permit`, so it needs both provider credentials: the API key serves
the API key and permit routes, and the user token serves the organization
member routes, which do not accept API keys. User tokens are short-lived (a
Keel dashboard session token expires after 30 minutes), so export a fresh one
before running the gate: `gate.sh` applies the saved plan right after
evaluating it, and the token must still be valid at apply time.

The pipeline is:

```sh
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json
opa eval -d policy.rego -i plan.json 'data.policy.deny'
terraform apply plan.tfplan
```

`policy.rego` denies new `keel_organization_member` resources that grant the `owner` role unless the plan includes an approval annotation in `owner_grant_approvals`, keyed by `org_id/user_id`.

Example approval input:

```hcl
member_role = "owner"

owner_grant_approvals = {
  "00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111" = {
    approved_by = "security@example.com"
    reason      = "Break-glass production access"
  }
}
```

Use this pattern when Terraform is the change surface but governance rules need to be enforced independently before infrastructure changes are applied.
