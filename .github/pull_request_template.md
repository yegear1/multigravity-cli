## Description

<!-- Provide a concise description of what this PR introduces or fixes. -->

## Changes

- Fix / feature item 1
- Fix / feature item 2

## Scope & Blast Radius

- [ ] New feature
- [ ] Bug fix
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] Breaking change (explain rationale and migration path below)

## Verification Checklist

Before submitting, please ensure the following commands pass locally:

- [ ] Unit tests pass: `go test -v ./...`
- [ ] Binary builds cleanly: `go build -o bin/multigravity ./cmd/multigravity`
- [ ] Shell scripts pass syntax check: `bash -n multigravity install.sh uninstall.sh`
- [ ] Whitespace and formatting clean: `git diff --check`
- [ ] New or modified code includes appropriate unit / characterization tests
- [ ] Backward compatibility maintained for existing profile data in `~/AntigravityProfiles`
