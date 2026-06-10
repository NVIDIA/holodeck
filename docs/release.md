# Releasing Holodeck

Holodeck releases are driven by [GoReleaser][goreleaser] and triggered by
pushing a `v*` git tag. The pipeline cross-builds the CLI (linux/darwin ×
amd64/arm64) and the action binary (linux × amd64/arm64) with
`CGO_ENABLED=0`, packages each binary as a tar.gz (bundling `LICENSE`
plus the README for the CLI), computes SHA256 checksums, publishes
everything to the GitHub Release for the tag, and bumps **two**
tap artifacts in lockstep:

- `Casks/holodeck.rb` — Homebrew **Cask** for macOS (arm64 + amd64).
  Casks bypass brew's Formula build-sandbox, which avoids the
  `PTY.open` failure that breaks Formula installs on macOS Tahoe
  (26.x) + brew 5.1.x + portable-ruby 4.0.x.
- `Formula/holodeck.rb` — Homebrew **Formula** for Linux (arm64 +
  amd64). The Formula carries `depends_on :linux` so brew refuses to
  install it on macOS even if a user tries `--formula`.

`brew install nvidia/holodeck/holodeck` resolves to the Cask on macOS
and the Formula on Linux automatically.

[goreleaser]: https://goreleaser.com/

## One-time setup

### `HOMEBREW_TAP_GITHUB_TOKEN` repo secret

The release workflow needs a Personal Access Token (PAT) with
`contents: write` scope on `NVIDIA/holodeck` to open the formula-bump
PR. The workflow's default `GITHUB_TOKEN` cannot do this because:

- Main branch protection requires PR review (default token can't
  approve its own PR).
- Main branch protection requires signed commits — commits authored via
  the GitHub API (which a PAT enables) are server-signed and satisfy
  this.
- Main branch protection requires DCO sign-off — the
  `commit_msg_template` in `.goreleaser.yaml` includes a
  `Signed-off-by:` trailer for the `nvidia-ci` identity to satisfy
  this. Without the trailer the formula-bump PR is opened but cannot
  merge.

**Setup steps:**

1. Go to the [fine-grained PAT settings page][pat-settings].
1. Resource owner: `NVIDIA`. Repository access: `Only select
   repositories` → `NVIDIA/holodeck`.
1. Repository permissions: `Contents: Read and write`, `Pull requests:
   Read and write`, `Metadata: Read` (auto-selected).
1. Expiration: 1 year (rotate per NVIDIA security policy).
1. Generate and copy the token.
1. Repo settings → Secrets and variables → Actions → New repository
   secret. Name it `HOMEBREW_TAP_GITHUB_TOKEN` and paste the PAT.

[pat-settings]: https://github.com/settings/personal-access-tokens/new

## Releasing a new version

### 1. Local dry-run

Before tagging, validate the build matrix and formula shape:

```bash
make snapshot
```

Inspect `dist/` — it should contain 6 archive tar.gz files plus
`checksums.txt` plus a source tarball. Both the generated
`dist/homebrew/Casks/holodeck.rb` (macOS-only, with the `postflight`
`xattr` hook) and `dist/homebrew/Formula/holodeck.rb` (Linux-only,
declares `depends_on :linux`) should be syntactically valid Ruby.

### 2. Tag and push

```bash
git tag -s vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Watch the release workflow on the [Actions tab][actions].

[actions]: https://github.com/NVIDIA/holodeck/actions

### 3. Verify the release

When the workflow completes, the [release page for the
tag][release-tag] should list the following assets:

```text
holodeck_X.Y.Z_linux_amd64.tar.gz
holodeck_X.Y.Z_linux_arm64.tar.gz
holodeck_X.Y.Z_darwin_amd64.tar.gz
holodeck_X.Y.Z_darwin_arm64.tar.gz
holodeck-action_X.Y.Z_linux_amd64.tar.gz
holodeck-action_X.Y.Z_linux_arm64.tar.gz
checksums.txt
holodeck-X.Y.Z.tar.gz   (source archive, auto-attached)
```

Two PRs (or two direct commits, depending on whether branch protection
forces PR mode for the `nvidia-ci` PAT) should also land on `main`:

- `chore(brew): bump holodeck cask to vX.Y.Z`    — touches `Casks/holodeck.rb`
- `chore(brew): bump holodeck formula to vX.Y.Z` — touches `Formula/holodeck.rb`

Review and merge any that aren't direct commits.

[release-tag]: https://github.com/NVIDIA/holodeck/releases

### 4. Post-release smoke test

On a clean machine (or with a fresh Homebrew prefix), verify the install
works end-to-end. Do this at minimum on macOS arm64 — Linux and macOS
amd64 are good-to-haves but not blocking.

```bash
brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck
# macOS:
brew install --cask nvidia/holodeck/holodeck
# Linux:
brew install --formula nvidia/holodeck/holodeck
holodeck --version
```

Install should complete in under 30 seconds (no Go toolchain build),
and `holodeck --version` should print `holodeck version vX.Y.Z`. On
macOS the Cask's `postflight` hook removes the `com.apple.quarantine`
xattr automatically — no manual `xattr -d` required.

If install fails, common causes: the cask/formula bump hasn't landed on
main yet (users hit the previous version); the cask/formula audit
failed (check the `homebrew-validate` workflow on the bump PR); an
archive URL returns 404 (the release wasn't fully published — re-run
the workflow); or, if `holodeck` is killed on first launch by
Gatekeeper, the `postflight` hook didn't run (re-install with
`HOMEBREW_NO_INSTALL_FROM_API=1 brew reinstall` and inspect output).

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

**`make snapshot` fails with "release notes are required":** add
`--skip=announce` to the snapshot command (already done in the
Makefile), and ensure your local git has at least one tag.

**Formula/Cask bump not landing on main:** check that
`HOMEBREW_TAP_GITHUB_TOKEN` is set and not expired. Check the release
workflow logs for the `homebrew formula` and `homebrew cask` step
output.

**`brew install` builds from source instead of using the binary:** the
formula isn't pointing at a valid archive URL. Inspect
`Formula/holodeck.rb` (Linux) or `Casks/holodeck.rb` (macOS) and
confirm the URL for your platform returns 200.

**macOS install fails with `can't get Master/Slave device`:** the user
is hitting the Tahoe brew Formula sandbox bug. Confirm they're on the
Cask path (`brew info --cask nvidia/holodeck/holodeck` should show the
cask). If brew picked the Formula instead, `brew uninstall holodeck &&
brew install --cask nvidia/holodeck/holodeck` forces the Cask.
