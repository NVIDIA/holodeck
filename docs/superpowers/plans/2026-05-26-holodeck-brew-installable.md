# Holodeck Homebrew Installability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship holodeck via Homebrew with a single `brew install` command, backed by a GoReleaser-driven release pipeline that auto-maintains the formula on every tag.

**Architecture:** Same-repo Homebrew tap — `Formula/holodeck.rb` lives in `NVIDIA/holodeck` main branch. GoReleaser cross-builds binaries on each `v*` tag push, attaches archives to the GitHub Release, and opens a PR updating the formula. PR mode (not direct commit) is required because main has required PR reviews + required signed commits — commits via the GitHub API are server-signed and satisfy that constraint.

**Tech Stack:** GoReleaser v2, GitHub Actions, Homebrew (custom-URL tap), Go 1.25, bash.

**Spec:** `docs/superpowers/specs/2026-05-26-holodeck-brew-installable-design.md`

---

## File Inventory

**Files created:**
- `.goreleaser.yaml` — release build + archive + formula generation config
- `.github/workflows/release.yaml` — tag-triggered GoReleaser workflow
- `.github/workflows/homebrew-validate.yaml` — formula lint on Formula/** changes
- `docs/release.md` — release process + post-release smoke-test checklist

**Files modified:**
- `cmd/cli/main.go` — replace hardcoded version with `var version` for ldflag injection
- `Makefile` — remove `release:` target, add `snapshot:` target, update `.PHONY` list
- `README.md` — add "Install via Homebrew" subsection

**Files NOT created in this plan** (GoReleaser creates them in the first release run via PR):
- `Formula/holodeck.rb` — created by GoReleaser's `brews:` block

---

## Task 1: Refactor version variable in cmd/cli/main.go

**Why:** The current hardcoded `c.Version = "0.2.18"` is stale and doesn't reflect the actual tag. We need a package-level `version` variable that ldflags can override at build time.

**Files:**
- Modify: `cmd/cli/main.go:97`

- [ ] **Step 1: Add the `version` package variable**

In `cmd/cli/main.go`, immediately after the `import` block closes (around line 37) and before `func main()`, add:

```go
// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"
```

- [ ] **Step 2: Replace the hardcoded version assignment**

Find this line in `cmd/cli/main.go:97`:

```go
	c.Version = "0.2.18"
```

Replace with:

```go
	c.Version = version
```

- [ ] **Step 3: Build and verify**

Run:
```bash
go build -o /tmp/holodeck-dev ./cmd/cli
/tmp/holodeck-dev --version
```

Expected output: `holodeck version dev`

Then test the ldflag injection:
```bash
go build -ldflags "-X main.version=v0.3.5" -o /tmp/holodeck-v035 ./cmd/cli
/tmp/holodeck-v035 --version
```

Expected output: `holodeck version v0.3.5`

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go
git commit -sS -m "refactor(cli): use package variable for version

Replace the hardcoded \"0.2.18\" version string with a package-level
\`version\` variable so GoReleaser can inject the real tag via
-ldflags -X main.version=<tag> at build time."
```

---

## Task 2: Add .goreleaser.yaml

**Why:** Defines the release build matrix, archive layout, checksum, source archive, and the `brews:` block that writes `Formula/holodeck.rb` to the same repo via PR.

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create the GoReleaser config**

Create `.goreleaser.yaml` with exactly this content:

```yaml
# Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
# Licensed under the Apache License, Version 2.0.
#
# GoReleaser config for cross-building holodeck binaries, publishing
# them as GitHub Release assets, and auto-updating the Homebrew formula
# at Formula/holodeck.rb on every v* tag.
#
# See docs/release.md for the release process.

version: 2

project_name: holodeck

before:
  hooks:
    - go mod tidy

builds:
  - id: holodeck
    main: ./cmd/cli
    binary: holodeck
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .Commit }}
      - -X main.date={{ .Date }}

  - id: holodeck-action
    main: ./cmd/action
    binary: holodeck-action
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w

archives:
  - id: holodeck
    ids:
      - holodeck
    name_template: "holodeck_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    files:
      - LICENSE
      - README.md

  - id: holodeck-action
    ids:
      - holodeck-action
    name_template: "holodeck-action_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    files:
      - LICENSE

checksum:
  name_template: checksums.txt
  algorithm: sha256

source:
  enabled: true

changelog:
  use: github
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^chore:'
      - '^test:'

release:
  draft: false
  prerelease: auto

brews:
  - name: holodeck
    ids:
      - holodeck
    repository:
      owner: NVIDIA
      name: holodeck
      branch: main
      pull_request:
        enabled: true
        draft: false
        base:
          branch: main
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/NVIDIA/holodeck"
    description: "Tool for creating and managing GPU-ready cloud test environments"
    license: "Apache-2.0"
    commit_author:
      name: nvidia-ci
      email: nvidia-ci@users.noreply.github.com
    commit_msg_template: "chore(brew): bump {{ .ProjectName }} to {{ .Tag }}"
    install: |
      bin.install "holodeck"
    test: |
      system "#{bin}/holodeck", "--version"
```

- [ ] **Step 2: Install GoReleaser locally for snapshot validation**

Run:
```bash
# macOS
brew install goreleaser

# OR via Go
go install github.com/goreleaser/goreleaser/v2@latest
```

Verify:
```bash
goreleaser --version
```

Expected: version `2.x.x`.

- [ ] **Step 3: Validate the config file**

Run:
```bash
goreleaser check
```

Expected: `config is valid` (no errors).

If errors appear, they're usually YAML syntax — fix and re-run.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml
git commit -sS -m "chore(release): add GoReleaser configuration

Cross-builds holodeck CLI for linux/darwin x amd64/arm64 and the
action binary for linux x amd64/arm64. Produces tar.gz archives,
sha256 checksums, source tarball, and auto-generates a Homebrew
formula at Formula/holodeck.rb via PR mode (required because main
has branch protection with required signed commits).

The PR mode token is read from \$HOMEBREW_TAP_GITHUB_TOKEN at
release time — see docs/release.md for setup."
```

---

## Task 3: Replace `make release` with `make snapshot`

**Why:** The existing `release:` target (Makefile:99-113) is dead code — it builds binaries to `bin/` but never publishes them, and GoReleaser now owns this responsibility. We replace it with a `snapshot:` target that runs GoReleaser in dry-run mode for local validation before tagging.

**Files:**
- Modify: `Makefile:11` (`.PHONY` line)
- Modify: `Makefile:99-113` (replace `release:` target)

- [ ] **Step 1: Replace the `release:` target with `snapshot:`**

In `Makefile`, find lines 99-113 (the entire `release:` target including its trailing blank line). The current content is:

```makefile
release:
	@rm -rf bin
	@mkdir -p bin
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "Building $$os-$$arch"; \
			GOOS=$$os GOARCH=$$arch $(GO_CMD) build -o bin/$(BINARY_NAME)-action-$$os-$$arch cmd/action/main.go; \
		done; \
	done
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "Building $$os-$$arch"; \
			GOOS=$$os GOARCH=$$arch $(GO_CMD) build -o bin/$(BINARY_NAME)-$$os-$$arch cmd/cli/main.go; \
		done; \
	done
```

Replace it with:

```makefile
# snapshot runs GoReleaser in dry-run mode: cross-builds binaries,
# produces archives + a draft Formula/holodeck.rb under ./dist, but
# does NOT publish to GitHub or push the formula. Use this before
# tagging a release to validate the build matrix and formula shape.
snapshot:
	goreleaser release --snapshot --clean --skip=publish
```

- [ ] **Step 2: Update the `.PHONY` declaration**

In `Makefile:11`, find:

```makefile
.PHONY: build fmt verify release lint vendor mod-tidy mod-vendor mod-verify check-vendor mdlint
```

Replace with:

```makefile
.PHONY: build fmt verify snapshot lint vendor mod-tidy mod-vendor mod-verify check-vendor mdlint
```

(`release` → `snapshot`.)

- [ ] **Step 3: Verify the snapshot target works**

Run:
```bash
make snapshot
```

Expected: GoReleaser runs, outputs go to `./dist/`, no upload. Final line is `release succeeded after Ns`.

Inspect the output:
```bash
ls dist/
```

Expected files (snapshot-flavored names):
- `holodeck_*_linux_amd64.tar.gz`
- `holodeck_*_linux_arm64.tar.gz`
- `holodeck_*_darwin_amd64.tar.gz`
- `holodeck_*_darwin_arm64.tar.gz`
- `holodeck-action_*_linux_amd64.tar.gz`
- `holodeck-action_*_linux_arm64.tar.gz`
- `checksums.txt`
- `holodeck-*.tar.gz` (source archive)
- A draft `holodeck.rb` somewhere in `dist/homebrew/` or printed in logs

Verify the formula shape:
```bash
find dist -name 'holodeck.rb' -exec cat {} \;
```

Expected: Ruby class definition with `on_macos`/`on_linux` blocks, each pinning a URL + SHA256. Verify the URLs reference `github.com/NVIDIA/holodeck/releases/download/...`.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -sS -m "build(make): replace release target with snapshot

The previous \`make release\` target cross-built binaries to bin/ but
never published them — dead code now that GoReleaser handles the
release flow.

\`make snapshot\` is the new local dry-run: runs GoReleaser in
--snapshot mode to validate the build matrix and inspect the
generated formula before tagging."
```

---

## Task 4: Add the release workflow

**Why:** GitHub Actions workflow that runs GoReleaser on every `v*` tag push. Builds binaries, publishes the GitHub Release, and opens the formula-bump PR.

**Files:**
- Create: `.github/workflows/release.yaml`

- [ ] **Step 1: Create the workflow file**

Create `.github/workflows/release.yaml` with exactly this content:

```yaml
# Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
# Licensed under the Apache License, Version 2.0.

name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # publish GitHub Release + tag artifacts

jobs:
  goreleaser:
    name: GoReleaser
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0   # GoReleaser needs full history for the changelog

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

- [ ] **Step 2: Verify YAML syntax**

Run:
```bash
# If yq is available
yq eval . .github/workflows/release.yaml >/dev/null && echo "valid YAML"

# Or with python
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yaml'))" && echo "valid YAML"
```

Expected: `valid YAML` printed, no exception.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yaml
git commit -sS -m "ci(release): add GoReleaser workflow on v* tags

Triggers on \`push: tags: v*\`. Runs goreleaser release --clean which
publishes binaries to the GitHub Release and opens a PR updating
Formula/holodeck.rb.

Uses the workflow's default GITHUB_TOKEN to publish the release.
The formula-bump PR is opened via a separate PAT
(HOMEBREW_TAP_GITHUB_TOKEN) — required because the default token
cannot open PRs against a protected branch from within the same
repo workflow context.

See docs/release.md for PAT setup."
```

---

## Task 5: Add the homebrew-validate workflow

**Why:** When `Formula/holodeck.rb` changes (e.g., GoReleaser bumps it), this workflow runs `brew audit --strict --online` and `brew style` to catch formula syntax errors before users hit them.

**Files:**
- Create: `.github/workflows/homebrew-validate.yaml`

- [ ] **Step 1: Create the validation workflow**

Create `.github/workflows/homebrew-validate.yaml` with exactly this content:

```yaml
# Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
# Licensed under the Apache License, Version 2.0.

name: homebrew-validate

on:
  pull_request:
    paths:
      - 'Formula/**'
  push:
    branches: [main]
    paths:
      - 'Formula/**'

permissions:
  contents: read

jobs:
  audit:
    name: Audit & Style
    runs-on: macos-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Audit formula
        run: |
          brew audit --strict --online Formula/holodeck.rb

      - name: Style check
        run: |
          brew style Formula/holodeck.rb
```

- [ ] **Step 2: Verify YAML syntax**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/homebrew-validate.yaml'))" && echo "valid YAML"
```

Expected: `valid YAML`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/homebrew-validate.yaml
git commit -sS -m "ci(brew): add formula audit + style workflow

Runs \`brew audit --strict --online\` and \`brew style\` against
Formula/holodeck.rb on PRs that touch Formula/** and on pushes to
main. Path-filtered so it stays dormant until the first formula
lands.

Catches formula syntax errors before they reach users."
```

---

## Task 6: Add Homebrew install section to README

**Why:** Users need to know the install command. The custom-URL `brew tap` form is required since the formula lives in `NVIDIA/holodeck` instead of `NVIDIA/homebrew-holodeck`.

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read the current README install area**

Run:
```bash
grep -n -A 8 "Quick Start\|make build" README.md | head -30
```

Note the exact location of the `make build` block. The new "Install via Homebrew" subsection goes immediately above it.

- [ ] **Step 2: Add the Homebrew install section**

In `README.md`, find this block (it appears around the "🚀 Quick Start" section):

```markdown
See [docs/quick-start.md](docs/quick-start.md) for a full walkthrough.

```bash
make build
sudo mv ./bin/holodeck /usr/local/bin/holodeck
holodeck --help
```
```

Replace it with:

```markdown
See [docs/quick-start.md](docs/quick-start.md) for a full walkthrough.

### Install via Homebrew (macOS, Linux)

```bash
brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck
brew install nvidia/holodeck/holodeck
holodeck --help
```

Pre-built binaries for macOS (arm64, amd64) and Linux (arm64, amd64) are
downloaded from the [GitHub Releases page](https://github.com/NVIDIA/holodeck/releases/latest).
Run `brew upgrade nvidia/holodeck/holodeck` to update.

### Install from source

```bash
make build
sudo mv ./bin/holodeck /usr/local/bin/holodeck
holodeck --help
```
```

- [ ] **Step 3: Verify the change**

Run:
```bash
grep -n "Install via Homebrew\|Install from source" README.md
```

Expected: two matches, both line numbers reasonable (in the install area, not at end of file).

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -sS -m "docs(readme): add Homebrew install instructions

Documents the \`brew tap nvidia/holodeck <url> && brew install\`
flow. Retains the existing make-build path as 'Install from source'
for contributors.

The custom-URL tap form is required because the formula lives in
NVIDIA/holodeck rather than a dedicated homebrew-* repo."
```

---

## Task 7: Add docs/release.md

**Why:** Documents the release process for maintainers — including the PAT setup (gating the whole pipeline) and the post-release smoke-test checklist.

**Files:**
- Create: `docs/release.md`

- [ ] **Step 1: Create the release doc**

Create `docs/release.md` with exactly this content:

````markdown
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
````

- [ ] **Step 2: Verify**

Run:
```bash
ls -la docs/release.md
wc -l docs/release.md
```

Expected: file exists, ~120 lines.

- [ ] **Step 3: Commit**

```bash
git add docs/release.md
git commit -sS -m "docs(release): add release process documentation

Documents the GoReleaser-driven release flow, PAT setup for
HOMEBREW_TAP_GITHUB_TOKEN, dry-run via \`make snapshot\`, tag-and-push
ritual, post-release smoke test, and rollback procedure for bad
releases."
```

---

## Task 8: Open the PR, get review, merge

**Files:** N/A (process step)

- [ ] **Step 1: Push the branch**

```bash
git push -u origin <current-branch>
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create --draft \
  --title "feat(release): make holodeck installable via Homebrew" \
  --body "$(cat <<'EOF'
Implements `docs/superpowers/specs/2026-05-26-holodeck-brew-installable-design.md`.

## Summary

- Adds GoReleaser config + release workflow on `v*` tags
- Adds homebrew-validate workflow (audit + style on Formula/** changes)
- Refactors `cmd/cli/main.go` to use a `version` package variable for ldflag injection (fixes stale hardcoded `0.2.18`)
- Replaces `make release` (dead code) with `make snapshot` for local dry-runs
- Adds README "Install via Homebrew" section
- Adds `docs/release.md` with the full release process + PAT setup

The first GoReleaser run (triggered by tagging `v0.3.5` after this merges) will create `Formula/holodeck.rb` via PR.

## Testing done

- \`goreleaser check\` → passes
- \`make snapshot\` → produces 6 archives + checksums + source + draft formula in dist/
- \`go build -ldflags "-X main.version=v0.3.5" ./cmd/cli && ./holodeck --version\` → prints \`holodeck version v0.3.5\`
- YAML syntax validated for both new workflows

## Required before merge

- [ ] Create \`HOMEBREW_TAP_GITHUB_TOKEN\` repo secret per \`docs/release.md\`. Without it, the v0.3.5 release workflow will fail at the formula-bump step (the release itself still publishes).

## Follow-ups (out of scope)

- Cut v0.3.5 to exercise the pipeline (separate task after merge)
- Post-release smoke test (separate task)
- homebrew-core submission (future)
- Signed binaries (cosign/sigstore — future)
EOF
)"
```

- [ ] **Step 3: Wait for required checks (DCO, CodeQL) to pass, then mark ready for review**

```bash
gh pr ready
```

- [ ] **Step 4: Address review feedback if any, then squash-merge**

```bash
gh pr merge --squash --auto
```

---

## Task 9: One-time PAT setup (manual, repo admin only)

**Files:** N/A (GitHub UI / settings)

This task **must be completed before tagging v0.3.5**, otherwise the formula-bump step will fail. The release itself will still publish — only the formula PR won't open.

- [ ] **Step 1: Generate a fine-grained PAT**

Follow the exact steps in `docs/release.md` → "One-time setup" → "`HOMEBREW_TAP_GITHUB_TOKEN` repo secret".

- [ ] **Step 2: Verify the secret exists**

```bash
gh secret list --repo NVIDIA/holodeck | grep HOMEBREW_TAP_GITHUB_TOKEN
```

Expected: one line showing the secret name and last-updated timestamp.

---

## Task 10: Cut v0.3.5 — the bootstrap release

**Files:** N/A (release operation)

- [ ] **Step 1: Update local main**

```bash
git checkout main
git pull origin main
```

Verify the PR from Task 8 is merged (all the new files are present):

```bash
ls .goreleaser.yaml .github/workflows/release.yaml docs/release.md
```

Expected: all three exist.

- [ ] **Step 2: Final dry-run sanity check**

```bash
make snapshot
```

Inspect the generated formula in `dist/`:

```bash
find dist -name 'holodeck.rb' -exec cat {} \;
```

GoReleaser's snapshot mode synthesizes a fake version (typically the latest
tag + `-next` suffix, so `0.3.4-next-SNAPSHOT-<sha>` or similar) — URLs
will NOT say `v0.3.5` yet. What to verify:

- Formula has `on_macos` × `on_arm`/`on_intel` and `on_linux` × `on_arm`/`on_intel` blocks
- URLs point at `github.com/NVIDIA/holodeck/releases/download/...`
- Each block has a non-empty `sha256`
- `install do bin.install "holodeck" end` and `test do system "#{bin}/holodeck", "--version" end` blocks present

If any of these are wrong, abort and fix the `.goreleaser.yaml` before tagging.

- [ ] **Step 3: Tag and push**

```bash
git tag -s v0.3.5 -m "Release v0.3.5: first Homebrew-installable release"
git push origin v0.3.5
```

- [ ] **Step 4: Watch the release workflow**

```bash
gh run watch
```

Or:
```bash
open https://github.com/NVIDIA/holodeck/actions/workflows/release.yaml
```

Expected: workflow completes green in ~3-5 minutes.

- [ ] **Step 5: Verify the release**

```bash
gh release view v0.3.5 --json assets | jq '.assets[].name'
```

Expected output (8 lines):
```
checksums.txt
holodeck-action_0.3.5_linux_amd64.tar.gz
holodeck-action_0.3.5_linux_arm64.tar.gz
holodeck_0.3.5_darwin_amd64.tar.gz
holodeck_0.3.5_darwin_arm64.tar.gz
holodeck_0.3.5_linux_amd64.tar.gz
holodeck_0.3.5_linux_arm64.tar.gz
holodeck-0.3.5.tar.gz
```

- [ ] **Step 6: Review and merge the formula-bump PR**

```bash
gh pr list --search "chore(brew): bump holodeck to v0.3.5" --json number,url
```

Open the PR in the browser, confirm `Formula/holodeck.rb` shape matches the snapshot output, then:

```bash
gh pr merge <PR_NUMBER> --squash
```

- [ ] **Step 7: Verify the homebrew-validate workflow ran green on the merge**

```bash
gh run list --workflow=homebrew-validate.yaml --limit 1
```

Expected: status `completed`, conclusion `success`.

---

## Task 11: Post-release smoke test

**Files:** N/A (manual verification)

- [ ] **Step 1: Clean macOS arm64 test**

On a macOS arm64 machine (or fresh `brew` environment):

```bash
# Optional: clear any cached tap state
brew untap nvidia/holodeck 2>/dev/null || true

brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck
brew install nvidia/holodeck/holodeck
holodeck --version
```

Expected output:
```
holodeck version v0.3.5
```

Time-to-install should be <30 seconds.

- [ ] **Step 2: Test the binary runs**

```bash
holodeck --help
```

Expected: usage output with all subcommands listed, exit code 0.

- [ ] **Step 3: (Optional) Repeat on macOS amd64 and Linux**

Same commands. Document any platform-specific failures in a follow-up issue.

- [ ] **Step 4: Mark the rollout complete**

Update the spec's status from `Draft (spec)` to `Implemented`:

```bash
sed -i.bak 's/^\*\*Status:\*\* Draft (spec)$/\*\*Status:\*\* Implemented/' \
  docs/superpowers/specs/2026-05-26-holodeck-brew-installable-design.md
rm docs/superpowers/specs/2026-05-26-holodeck-brew-installable-design.md.bak

git add docs/superpowers/specs/2026-05-26-holodeck-brew-installable-design.md
git commit -sS -m "docs(spec): mark holodeck-brew-installable as Implemented

v0.3.5 ships via \`brew install nvidia/holodeck/holodeck\`.
Smoke-tested on macOS arm64."
git push
```

---

## Verification commands (full checklist)

After all tasks complete, the following must all succeed:

| Command | Expected |
|---|---|
| `goreleaser check` | `config is valid` |
| `make snapshot` | Produces 6 archives + checksums + source + draft formula |
| `go build -ldflags "-X main.version=v0.3.5" ./cmd/cli && ./holodeck --version` | `holodeck version v0.3.5` |
| `gh release view v0.3.5 --json assets \| jq '.assets \| length'` | `8` |
| `brew tap nvidia/holodeck https://github.com/NVIDIA/holodeck && brew install nvidia/holodeck/holodeck && holodeck --version` (clean machine) | `holodeck version v0.3.5` |
| `gh run list --workflow=homebrew-validate.yaml --limit 1 --json conclusion` | `"success"` |

---

## Notes for the implementing engineer

- **TDD note:** This work is mostly config and CI — there's no meaningful unit test surface for `.goreleaser.yaml` or workflow YAML. The "tests" are `goreleaser check`, `make snapshot`, YAML parse checks, and the post-release smoke test. The only Go code change (Task 1) is verified by running the binary, not by a Go unit test — adding a unit test for a 2-line variable refactor would be theater per the constitution.

- **Signed commits:** All commits use `-sS` (DCO sign-off + GPG signature) per the repo's branch-protection requirement.

- **PAT scope:** The `HOMEBREW_TAP_GITHUB_TOKEN` PAT only needs `Contents: write` and `Pull requests: write` on `NVIDIA/holodeck` specifically. Do not grant broader scope.

- **If a task fails:** Stop, diagnose, fix. Do not skip ahead — Task 10 (the actual release) depends on every prior task landing correctly.

- **Plan execution method:** Solo path is fine — this is a single-engineer change with no parallel decomposition opportunity. Most tasks are 5-15 minutes of file editing + a verification command.
