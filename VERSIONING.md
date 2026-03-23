# Versioning

Syllago follows [Semantic Versioning 2.0.0](https://semver.org/).

## Pre-1.0 Convention

Syllago is pre-1.0. During this period:

- **Minor version** (0.X.0) -- additive releases with new features and providers
- **Patch version** (0.X.Y) -- bug fixes, CI/tooling changes, no new features
- **Breaking changes** -- can occur in any minor release; documented explicitly in release notes and CHANGELOG.md

## Post-1.0 Convention (Planned)

After 1.0.0:

- **Major version** -- breaking changes to CLI commands, config format, or content format
- **Minor version** -- new features, new providers, additive changes
- **Patch version** -- bug fixes only

## Release Process

1. Assess commits since last release; determine release type (major/minor/patch)
2. Write release notes in `releases/vX.Y.Z.md`
3. Bump the `VERSION` file
4. Run the release checklist (below)
5. Commit: `release: prepare vX.Y.Z -- <highlights>`
6. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z: <highlights>"`
7. Push tag -- the GitHub Actions release workflow builds binaries, checksums, cosign signs, creates the GitHub Release, and updates the Homebrew formula

The `/release` skill automates steps 1-6 interactively.

## Release Checklist

Before tagging:

- [ ] `releases/vX.Y.Z.md` exists with highlights, features, fixes, and breaking changes
- [ ] `VERSION` file updated
- [ ] `commands.json` is fresh: `syllago _gendocs > commands.json && git diff commands.json`
- [ ] `make test` passes locally
- [ ] CI is green on main
- [ ] Breaking changes documented in release notes and CHANGELOG.md
- [ ] `go install` works: `go install github.com/OpenScribbler/syllago/cli/cmd/syllago@vX.Y.Z` (requires public repo)

## Version Discovery

```bash
syllago version          # Print installed version
syllago version --json   # JSON output with version, commit, and build date
```
