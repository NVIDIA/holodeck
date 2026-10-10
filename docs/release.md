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

The release workflow needs a fine-grained Personal Access Token (PAT)
on `NVIDIA/holodeck` to push the tap-bump branches and open the bump
PRs. GoReleaser never writes to `main` with it: each bump is committed
to a per-release branch and merged through a normal PR (see
[Tap-bump PRs](#tap-bump-prs)), so the PAT needs only `Contents` and
`Pull requests` write access. It does **not** need to belong to a repo
admin, and must not rely on the admin bypass of `main`'s branch
protection.

The workflow's default `GITHUB_TOKEN` is not used for this because
pull requests opened with it do not trigger other workflows, so the
required `DCO`, `Build`, and CodeQL checks would never run on the bump
PRs.

**Setup steps:**

1. Go to the [fine-grained PAT settings page][pat-settings].
1. Resource owner: `NVIDIA`. Repository access: `Only select
   repositories` → `NVIDIA/holodeck`.
1. Repository permissions: `Contents: Read and write`, `Pull requests:
   Read and write`, `Metadata: Read` (auto-selected).
1. Expiration: 1 year (rotate per NVIDIA security policy). The PAT
   owner needs write access to the repo; admin is not required.
1. Generate and copy the token.
1. Repo settings → Secrets and variables → Actions → New repository
   secret. Name it `HOMEBREW_TAP_GITHUB_TOKEN` and paste the PAT.

[pat-settings]: https://github.com/settings/personal-access-tokens/new

### Tap-bump PRs

For a stable tag `vX.Y.Z`, GoReleaser (via the GitHub Contents API, as
configured in the `brews:` and `homebrew_casks:` blocks of
`.goreleaser.yaml`):

1. Creates branch `brew/holodeck-formula-X.Y.Z` from `main`, commits
   `Formula/holodeck.rb` to it, and opens a PR into `main`.
1. Creates branch `brew/holodeck-cask-X.Y.Z` from `main`, commits
   `Casks/holodeck.rb` to it, and opens a PR into `main`.

Properties of those commits:

- **Author and committer** are `nvidia-ci
  <nvidia-ci@users.noreply.github.com>` (`commit_author` in
  `.goreleaser.yaml`).
- **DCO**: `commit_msg_template` ends with a `Signed-off-by:` trailer
  for the same `nvidia-ci` identity, so the `DCO` check passes.
- **Not signed.** GitHub signs Contents API commits only when the
  request is authenticated as a GitHub App or bot and carries no custom
  author or committer. GoReleaser authenticates with the PAT and sets
  `nvidia-ci` as committer, so the commit is unsigned (the v0.3.6
  bumps, `3c492cfe` and `ad15197f`, report `verification.reason:
  unsigned`). `main` requires signed commits, and GitHub checks every
  commit on the head branch, so an unsigned bump commit can block the
  PR even with **Squash and merge**. Re-sign it before merging, as
  described in step 3 of [Releasing a new version](#releasing-a-new-version).

Prerelease tags (anything with a semver prerelease suffix, such as
`vX.Y.Z-rc.1`) publish the GitHub Release but skip both tap bumps
(`skip_upload: auto`), so `brew install` keeps resolving to the latest
stable version.

Through v0.4.0 both blocks set `branch: main`. When the branch equals
`pull_request.base.branch`, GoReleaser commits directly to `main` and
the PR it then tries to open is rejected as empty, which only worked
because the PAT owner could bypass branch protection as an admin.

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

The tag must reach `NVIDIA/holodeck` itself, so push it to the remote that
points there. In a fork-based checkout that remote is usually `upstream`,
not `origin`. A tag pushed to a fork does not trigger the release workflow.
Tag the tip of `NVIDIA/holodeck` `main`:

```bash
git fetch upstream
git tag -s vX.Y.Z -m "Release vX.Y.Z" upstream/main
git push upstream vX.Y.Z
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

Two PRs into `main` should also be open (see
[Tap-bump PRs](#tap-bump-prs)):

- `chore(brew): bump holodeck cask to vX.Y.Z` from
  `brew/holodeck-cask-X.Y.Z`, touching `Casks/holodeck.rb`
- `chore(brew): bump holodeck formula to vX.Y.Z` from
  `brew/holodeck-formula-X.Y.Z`, touching `Formula/holodeck.rb`

Each bump commit is unsigned (see [Tap-bump PRs](#tap-bump-prs)), so
a maintainer re-signs it before merging. Amending keeps `nvidia-ci` as
the author, so the existing `Signed-off-by:` trailer still satisfies
DCO; the maintainer becomes the committer and signs. The bump branches
exist only on `NVIDIA/holodeck`, so fetch and push them through the
remote that points there (`upstream` in a fork-based checkout, as in
step 2):

```bash
git fetch upstream brew/holodeck-cask-X.Y.Z
git switch -c brew/holodeck-cask-X.Y.Z upstream/brew/holodeck-cask-X.Y.Z
git commit --amend --no-edit -S
git push --force-with-lease upstream brew/holodeck-cask-X.Y.Z
# repeat for brew/holodeck-formula-X.Y.Z
```

Then wait for `DCO`, `Build`, CodeQL, and `homebrew-validate` to pass,
get the one required review, squash-merge each PR, and delete its
branch. Users keep getting the previous version until both are merged.

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
git push upstream :refs/tags/vX.Y.Z
git tag -d vX.Y.Z

# Close the auto-opened tap-bump PRs without merging, deleting their branches
gh pr close <CASK_PR_NUMBER> --delete-branch
gh pr close <FORMULA_PR_NUMBER> --delete-branch
```

Then fix the underlying issue and re-tag.

## Troubleshooting

**`make snapshot` fails with "release notes are required":** add
`--skip=announce` to the snapshot command (already done in the
Makefile), and ensure your local git has at least one tag.

**No tap-bump PRs after a release:** prerelease tags skip the bumps by
design. For a stable tag, check that `HOMEBREW_TAP_GITHUB_TOKEN` is set,
not expired, and has `Contents` and `Pull requests` write access. Check
the release workflow logs for the `homebrew formula` and `homebrew
cask` step output. If the `brew/holodeck-*-X.Y.Z` branch exists but no
PR does, open one manually from that branch into `main`.

**Tap-bump PR cannot be merged ("commits must have verified
signatures"):** the `nvidia-ci` commit on the bump branch was not
re-signed. Follow the re-sign steps in step 3 of
[Releasing a new version](#releasing-a-new-version).

**`brew install` builds from source instead of using the binary:** the
formula isn't pointing at a valid archive URL. Inspect
`Formula/holodeck.rb` (Linux) or `Casks/holodeck.rb` (macOS) and
confirm the URL for your platform returns 200.

**macOS install fails with `can't get Master/Slave device`:** the user
is hitting the Tahoe brew Formula sandbox bug. Confirm they're on the
Cask path (`brew info --cask nvidia/holodeck/holodeck` should show the
cask). If brew picked the Formula instead, `brew uninstall holodeck &&
brew install --cask nvidia/holodeck/holodeck` forces the Cask.
