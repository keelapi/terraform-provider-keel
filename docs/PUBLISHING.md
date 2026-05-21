# Publishing

This repository is being prepared for publication as the `keelapi/keel`
Terraform provider. The provider source is set in `main.go` as
`registry.terraform.io/keelapi/keel`.

## One-time GitHub repository rename

Christian should rename this GitHub repository in the GitHub UI when ready:

1. Open the repository on GitHub.
2. Go to **Settings** > **General** > **Repository name**.
3. Rename it to `terraform-provider-keel`.
4. Confirm the repository is public before publishing to the Terraform Registry.

The Terraform Registry detects public provider repositories whose names match
`terraform-provider-{NAME}`. For this provider, the GitHub repository name must
be `terraform-provider-keel` so the Registry provider address can be
`keelapi/keel`.

## One-time GPG signing key setup

Provider releases must include a detached GPG signature for the checksum file.
Do not commit generated keys or exported key files to this repository.

Generate a signing key:

```sh
gpg --full-generate-key
```

Use RSA-4096 for the most registry-compatible setup. HashiCorp's current
publishing docs note that the Registry signing-key API accepts RSA and DSA keys
but not the default ECC type, so RSA-4096 is safer than ed25519 for first
publication. If ed25519 is used, verify the Registry accepts the public key
before cutting the first release.

Find the key ID or fingerprint:

```sh
gpg --list-secret-keys --keyid-format=long
```

Export the public key for Terraform Registry upload:

```sh
gpg --armor --export <KEY-ID> > terraform-provider-keel-public.asc
```

Export the private key for the GitHub Actions secret:

```sh
gpg --armor --export-secret-keys <KEY-ID>
```

In GitHub, add repository secrets under **Settings** > **Secrets and variables**
> **Actions**:

- `GPG_PRIVATE_KEY`: the full ASCII-armored private key export.
- `PASSPHRASE`: the passphrase for the GPG key.

In the Terraform Registry, upload the ASCII-armored public key at
**User Settings** > **Signing Keys** for the `keelapi` namespace.

## First publish on registry.terraform.io

1. Confirm the GitHub repository is renamed to `terraform-provider-keel` and is
   public.
2. Confirm the `GPG_PRIVATE_KEY` and `PASSPHRASE` GitHub Actions secrets exist.
3. Confirm the public GPG key is uploaded to the Terraform Registry under the
   namespace that will publish `keelapi/keel`.
4. Create the first signed GitHub release by pushing a version tag.
5. After the GitHub release finishes, sign in to
   `https://registry.terraform.io` with the GitHub account that has access to
   the repository.
6. Choose **Publish** > **Provider** and select `keelapi/terraform-provider-keel`.

Publishing creates the Registry provider entry and configures a release webhook
so future GitHub releases can be ingested automatically.

## Cutting a release

After the one-time setup is complete, cut releases by tagging the commit and
pushing the tag:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The `.github/workflows/release.yml` workflow runs on tags matching `v*`. It
imports the signing key from GitHub secrets, runs `goreleaser release --clean`,
uploads zipped provider binaries, includes
`terraform-provider-keel_<VERSION>_manifest.json`, writes
`terraform-provider-keel_<VERSION>_SHA256SUMS`, and signs only the checksum file
as `terraform-provider-keel_<VERSION>_SHA256SUMS.sig`.
