#!/usr/bin/env bash
set -euo pipefail

failed=0

while IFS= read -r -d '' path; do
  [[ -f "$path" ]] || continue

  case "$(basename "$path")" in
    *private*.asc|*private*.gpg|*private*.key|*private*.pem)
      printf 'private-key filename is tracked: %s\n' "$path" >&2
      failed=1
      continue
      ;;
  esac

  if LC_ALL=C grep -aEq \
    '^(-----BEGIN PGP PRIVATE KEY BLOCK-----|-----BEGIN ([A-Z0-9 ]+ )?PRIVATE KEY-----|-----BEGIN OPENSSH PRIVATE KEY-----)$' \
    "$path"; then
    printf 'private-key material detected in tracked file: %s\n' "$path" >&2
    failed=1
  fi
done < <(git ls-files -z)

if (( failed != 0 )); then
  exit 1
fi

printf 'No tracked private-key material detected.\n'
