package policy

import rego.v1

deny contains msg if {
	change := input.resource_changes[_]
	change.type == "keel_organization_member"
	change.change.actions[_] == "create"

	after := change.change.after
	after.role == "owner"
	not approved_owner_grant(after.org_id, after.user_id)

	msg := sprintf("owner role grant for org %s user %s requires owner_grant_approvals[%q].approved_by", [after.org_id, after.user_id, grant_key(after.org_id, after.user_id)])
}

approved_owner_grant(org_id, user_id) if {
	approval := input.variables.owner_grant_approvals.value[grant_key(org_id, user_id)]
	approval.approved_by != ""
}

grant_key(org_id, user_id) := key if {
	key := sprintf("%s/%s", [org_id, user_id])
}
