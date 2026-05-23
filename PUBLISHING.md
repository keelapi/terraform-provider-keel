# Publishing

This is the Christian-side checklist for publishing the `keelapi/keel`
Terraform provider to the public Terraform Registry. Do not generate, export,
commit, or upload signing key material from automation or from another person's
machine.

## Current automation audit

The release automation is configured for Terraform Registry releases after the
one-time GPG and Registry setup is complete.

- `.github/workflows/release.yml` runs on pushed `v*` tags.
- The workflow imports `secrets.GPG_PRIVATE_KEY` with
  `secrets.PASSPHRASE`, then passes the imported key fingerprint to GoReleaser
  as the `GPG_FINGERPRINT` environment variable.
- `.goreleaser.yml` builds zip archives for the supported Terraform provider
  platforms, includes `terraform-registry-manifest.json`, writes
  `terraform-provider-keel_<VERSION>_SHA256SUMS`, and creates a detached
  GPG signature at `terraform-provider-keel_<VERSION>_SHA256SUMS.sig`.
- The checksum signature uses `gpg2 --batch --local-user
  {{ .Env.GPG_FINGERPRINT }} --output ${signature} --detach-sign ${artifact}`.

The workflow consumes these GitHub Actions secrets:

| Name | Required | Purpose |
| ---- | -------- | ------- |
| `GPG_PRIVATE_KEY` | Yes | ASCII-armored private key export for the Terraform Registry release signing key. |
| `PASSPHRASE` | Only if the key has a passphrase | Passphrase for the private key. This workflow uses the HashiCorp/GoReleaser convention `PASSPHRASE`, not `GPG_PASSPHRASE`. |

`GPG_FINGERPRINT` is an environment variable used by GoReleaser, but it is not a
GitHub secret in this workflow. The workflow derives it from the imported key
via `steps.import_gpg.outputs.fingerprint`. Record the fingerprint locally so
the uploaded public key and first release logs can be checked against it.

## 1. Generate the GPG keypair

Christian should perform this step locally on a trusted machine.

```sh
gpg --full-generate-key
```

Use these choices unless Keel has a stricter internal key policy:

- Key type: `RSA and RSA`
- Key size: `4096`
- User ID: a Keel release-signing identity, such as
  `Keel Terraform Registry Signing <security@keelapi.com>`
- Passphrase: use a strong passphrase and store it in the team password manager

The public Terraform Registry accepts RSA and DSA signing keys, but not the
default ECC key type, so RSA 4096 is the safest first-publication choice.

Find the fingerprint:

```sh
gpg --list-secret-keys --keyid-format=long --fingerprint
```

Set a shell variable for the full fingerprint, with spaces removed:

```sh
export KEY_FINGERPRINT="PASTE_FULL_FINGERPRINT_WITHOUT_SPACES"
```

Export the public key for Terraform Registry upload:

```sh
gpg --armor --export "$KEY_FINGERPRINT" > terraform-provider-keel-public.asc
```

Export the private key for the GitHub Actions secret:

```sh
gpg --armor --export-secret-keys "$KEY_FINGERPRINT" > terraform-provider-keel-private.asc
```

Treat `terraform-provider-keel-private.asc` as sensitive. Do not commit it. Do
not send it in chat. Remove it after the GitHub secret is confirmed, unless
Keel's internal key custody policy requires retaining a local encrypted copy.

## 2. Add GitHub Actions secrets

Add the secrets under the GitHub repository's **Settings > Secrets and
variables > Actions** page.

Current remote audit:

```text
keelapi/terraform-provider-keel
```

Add:

- `GPG_PRIVATE_KEY`: the complete contents of
  `terraform-provider-keel-private.asc`, including the
  `-----BEGIN PGP PRIVATE KEY BLOCK-----` and
  `-----END PGP PRIVATE KEY BLOCK-----` lines.
- `PASSPHRASE`: the key passphrase, if the key has one.

Do not add key files to the repository. GitHub's automatically provided
`GITHUB_TOKEN` is already used by the workflow and does not need to be added as
a repository secret.

## 3. Upload the public key to the Terraform Registry

Sign in at `https://registry.terraform.io` with the GitHub account that can
administer the `keelapi` namespace.

Open `https://registry.terraform.io/settings/gpg-keys`, choose the `keelapi`
namespace, and upload the complete contents of
`terraform-provider-keel-public.asc`.

Confirm the Registry shows the same fingerprint recorded in Step 1.

## 4. Confirm the Registry namespace and repository name

The public Terraform Registry only detects public GitHub provider repositories
whose names match `terraform-provider-{NAME}`. For the provider address
`keelapi/keel`, the GitHub repository must be public and named
`terraform-provider-keel`.

If the GitHub UI still shows `keelapi/keel-terraform`, rename the repository to
`keelapi/terraform-provider-keel` before publishing. The local `origin` remote
already points at `https://github.com/keelapi/terraform-provider-keel.git`.

Create or confirm the `keelapi` Registry namespace before submitting the
provider listing.

## 5. Submit the provider listing

Open `https://registry.terraform.io/publish/provider`.

1. Connect the GitHub account if prompted.
2. Select the `keelapi` namespace.
3. Select the `keelapi/terraform-provider-keel` repository.
4. Complete the provider publish flow.

Publishing creates the provider listing and a GitHub release webhook. Future
GitHub releases are detected by the Registry automatically.

## 6. Cut the next release tag

After the signing key, GitHub secrets, Registry public key, namespace, and
provider listing are ready, cut the next SemVer tag. The previous source release
is v1.0, so the expected first Registry ingestion tag is likely `v1.0.1` unless
the release plan chooses another bump.

```sh
git fetch --tags origin
git tag --list 'v1*'
git tag v1.0.1
git push origin v1.0.1
```

The release workflow will create the GitHub Release with:

- zipped provider binaries,
- `terraform-provider-keel_<VERSION>_manifest.json`,
- `terraform-provider-keel_<VERSION>_SHA256SUMS`,
- `terraform-provider-keel_<VERSION>_SHA256SUMS.sig`.

Do not modify or replace an already-published provider version. If a release
asset is wrong after publication, cut a new SemVer patch tag.

## Expected Christian hand-time

Budget 30 to 60 minutes:

- 10 to 15 minutes to generate, export, and record the GPG keypair.
- 5 to 10 minutes to add GitHub Actions secrets.
- 5 to 10 minutes to upload the public key to the Terraform Registry.
- 10 to 20 minutes to confirm namespace access, submit the provider listing,
  cut the tag, and watch the first release and Registry ingestion.

## References

- HashiCorp provider publishing docs:
  `https://developer.hashicorp.com/terraform/registry/providers/publishing`
- GoReleaser GitHub Actions signing docs:
  `https://goreleaser.com/customization/ci/actions/`
