# OPA Policy Gate

This example gates a Keel Terraform plan with Open Policy Agent before apply:

```sh
terraform init
./gate.sh
```

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
