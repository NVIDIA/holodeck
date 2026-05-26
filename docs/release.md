# Releasing Holodeck

Holodeck releases are driven by [GoReleaser](https://goreleaser.com/) and
triggered by pushing a `v*` git tag. The pipeline:

1. Cross-builds the CLI (linux/darwin × amd64/arm64) and the action binary
   (linux × amd64/arm64) with `CGO_ENABLED=0`.
2. Packages each binary as a tar.gz (bundles `LICENSE`, README for CLI).
3. Computes SHA256 checksums.
4. Publishes everything to the GitHub Release for the tag.
5. Opens a PR updating `Formula/holodeck.rb` with the new version, archive
   URLs, and SHA256 sums.

## One-time setup

### `HOMEBREW_TAP_GITHUB_TOKEN` repo secret

The release workflow needs a Personal Access Token (PAT) with
`contents: write` scope on `NVIDIA/holodeck` to open the formula-bump PR.
The workflow's default `GITHUB_TOKEN` cannot do this because:

- Main branch protection requires PR review (default token can't approve
  its own PR).
- Main branch protection requires signed commits — commits authored via
  the GitHub API (which a PAT enables) are server-signed and satisfy this.

**Setup steps:**

1. Go to https://github.com/settings/personal-access-tokens/new (fine-grained PAT).
2. Resource owner: `NVIDIA`. Repository access: `Only select repositories` → `NVIDIA/holodeck`.
3. Repository permissions:
   - `Contents`: Read and write
   - `Pull requests`: Read and write
   - `Metadata`: Read (auto-selected)
4. Expiration: 1 year (rotate per NVIDIA security policy).
5. Generate and copy the token.
6. Repo settings → Secrets and variables → Actions → New repository secret:
   - Name: `HOMEBREW_TAP_GITHUB_TOKEN`
   - Value: the PAT from step 5.

## Releasing a new version

### 1. Local dry-run

Before tagging, validate the build matrix and formula shape:

```bash
make snapshot
```

Inspect:

- `dist/` should contain 6 archive tar.gz files + `checksums.txt` + a source
  tarball.
- `dist/homebrew/Formula/holodeck.rb` (or wherever GoReleaser writes it —
  check the snapshot output logs) should be a syntactically valid formula.

### 2. Tag and push

```bash
git tag -s vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Watch the release workflow at https://github.com/NVIDIA/holodeck/actions.

### 3. Verify the release

When the workflow completes:

- https://github.com/NVIDIA/holodeck/releases/tag/vX.Y.Z should list:
  - `holodeck_X.Y.Z_linux_amd64.tar.gz`
  - `holodeck_X.Y.Z_linux_arm64.tar.gz`
  - `holodeck_X.Y.Z_darwin_amd64.tar.gz`
  - `holodeck_X.Y.Z_darwin_arm64.tar.gz`
  - `holodeck-action_X.Y.Z_linux_amd64.tar.gz`
  - `holodeck-action_X.Y.Z_linux_arm64.tar.gz`
  - `checksums.txt`
  - Source tarball (auto-attached)
- A new PR titled `chore(brew): bump holodeck to vX.Y.Z` should be open.
  Review and merge it.

### 4. Post-release smoke test

On a clean machine (or with a fresh Homebrew prefix), verify the install
works end-to-end. **Do this at minimum on macOS arm64 — Linux and macOS
amd64 are good-to-haves but not blocking.**

```bash
brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck
brew install nvidia/holodeck/holodeck
holodeck --version
```

Expected:

- Install completes in <30 seconds (no Go toolchain build).
- `holodeck --version` prints `holodeck version vX.Y.Z`.

If install fails, common causes:

- Formula-bump PR not merged yet → users hit the previous version.
- Formula audit failed → check the `homebrew-validate` workflow on the
  formula-bump PR.
- Archive URL 404 → release wasn't fully published; re-run the workflow.

### 5. Cleanup if a release goes wrong

```bash
# Delete the bad release + tag locally and remotely
gh release delete vX.Y.Z --yes
git push origin :refs/tags/vX.Y.Z
git tag -d vX.Y.Z

# Close the auto-opened formula-bump PR without merging
gh pr close <PR_NUMBER>
```

Then fix the underlying issue and re-tag.

## Troubleshooting

**`make snapshot` fails with "release notes are required"**
→ Add `--skip=announce` to the snapshot command (already done in the
Makefile), and ensure your local git has at least one tag.

**Formula PR not opened**
→ Check `HOMEBREW_TAP_GITHUB_TOKEN` is set and not expired.
→ Check the release workflow logs for the `brews` step output.

**`brew install` builds from source instead of using the binary**
→ The formula isn't pointing at a valid archive URL. Inspect
`Formula/holodeck.rb` and confirm the URL for your platform 200s.
