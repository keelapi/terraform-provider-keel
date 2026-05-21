#!/usr/bin/env sh
set -eu

terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json
opa eval -d policy.rego -i plan.json 'data.policy.deny'
opa eval --fail -d policy.rego -i plan.json 'count(data.policy.deny) == 0'
terraform apply plan.tfplan
