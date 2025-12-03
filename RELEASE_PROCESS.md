# Release Process

- [Breaking Changes](#breaking-changes)
- [Major Release Procedure](#major-release-procedure)
  - [Creating a new release branch](#creating-a-new-release-branch)
  - [Cutting a new release](#cutting-a-new-release)
  - [Tagging Procedure](#tagging-procedure)
- [Release Policy](#release-policy)
  - [Definitions](#definitions)
  - [Policy](#policy)
- [Non-major Release Procedure](#non-major-release-procedure)

This document outlines the release process for the Babylon node (babylond)

Babylon follows [semantic versioning](https://semver.org), but with the
following deviations to account for state-machine and API breaking changes.

- State-machine breaking changes will result in an increase of the major version X (X.y.z).
- Emergency releases & API breaking changes will result in an increase of the minor version Y (x.Y.z | x > 0).
- All other changes will result in an increase of the patch version Z (x.y.Z | x > 0).

## Breaking Changes

A change is considered to be ***state-machine breaking*** if it requires a
coordinated upgrade for the network to preserve state compatibility Note that
when bumping the dependencies of [Cosmos
SDK](https://github.com/cosmos/cosmos-sdk) and
[IBC](https://github.com/cosmos/ibc-go), we will only treat patch releases as
non state-machine breaking.

A change is considered to be ***API breaking*** if it modifies the provided API.
This includes events, queries, CLI interfaces.

## Major Release Procedure

A _major release_ is an increment of the first number (eg: `v9.1.0` → `v10.0.0`).

**Note**: Generally, PRs should target either `main` or a long-lived feature
branch (see [CONTRIBUTING.md](./CONTRIBUTING.md#pull-requests)).

* Once the team feels that `main` is _**feature complete**_, we create a
  `release/vY.x` branch (going forward known as release branch), where `Y` is
  the version number, with the minor and patch part substituted to `x` (eg: 11.x).
  * **PRs targeting directly a release branch can be merged _only_ when
    exceptional circumstances arise**.
* In the release branch
  * Create a PR that adds new version section in the `CHANGELOG.md`, matching the released version e.g
    for branch `release/vY.x`, section will be called `vY.0.0`
* We freeze the release branch from receiving any new features and focus on
  releasing a release candidate.
  * Finish audits and reviews.
  * Add more tests.
  * Fix bugs as they are discovered.
* After the team feels that the release branch works fine, we cut a release
  candidate.
  * Create a new annotated git tag for a release candidate in the release branch
    (follow the [Tagging Procedure](#tagging-procedure)).
  * The release verification on devnet must pass.
  * When bugs are found, create a PR for `main`, and backport fixes to the
    release branch.
  * Before tagging the release, create and merge PR to the release branch that:
    * Moves all changelog entries form `Unreleased` section of the changelog to the newly created section `vY.0`
  * Create new release candidate tags after bugs are fixed.
* After the team feels the release candidate is ready, create a full release:
  * **Note:** The final release MUST have the same commit hash as the latest
    corresponding release candidate.
  * Create a new annotated git tag in the release branch (follow the [Tagging Procedure](#tagging-procedure))
* After the final release is made e.g `vY.0.0`, backport changelog changes to the `main` branch
  * checkout a new branch from the main branch: `username/backport_changelog`
  * bring the new section from `release/vY.x` branch to the `CHANGELOG.md` file on `main` branch
  * open PR against the `main` branch

### Creating a new release branch

**For major releases** (e.g., `release/v3.x`, `release/v4.x`):
Create the release branch from `main`:

```bash
git checkout main
git pull
git checkout -b release/v4.x
git push
```

**For minor releases** (e.g., `release/v4.2.x` when `release/v4.1.x` exists):
Create the release branch from the previous minor release branch:

```bash
git checkout release/v4.1.x
git pull
git checkout -b release/v4.2.x
git push
```

### Cutting a new release

Before cutting a release (e.g., `v2.0.0-rc.0`), the
following steps are necessary:

- move to the release branch, e.g., `release/v2.x`
    ```bash
    git checkout release/v2.x
    ```
- create new tag (follow the [Tagging Procedure](#tagging-procedure))

### Tagging Procedure

**Important**: _**Always create tags from your local machine**_ since all
release tags should be signed and annotated. Using Github UI will create a
`lightweight` tag, so it's possible that `babylond version` returns a commit
hash, instead of a tag. This is important because most operators build from
source, and having incorrect information when you run `make install && babylond
version` raises confusion.

The following steps are the default for tagging a specific branch commit using
git on your local machine. Usually, release branches are labeled `release/v*`:

Ensure you have checked out the commit you wish to tag and then do (assuming
you want to release version `v2.0.0-rc.0` ):
```bash
git pull --tags

git tag -s -a v2.0.0-rc.0 -m "Version v2.0.0-rc.0"
```

With every tag, the Github action will use the `goreleaser` tool to create a
release, including artifacts and their checksums.

## Release Policy

### Definitions

A `major` release is an increment of the _major number_ (eg: `v9.X.X → v10.X.X`).

A `minor` release is an increment of the _minor number_ (eg: `v9.0.X → v9.1.X`).

A `patch` release is an increment of the _patch number_ (eg: `v10.0.0` → `v10.0.1`).

### Policy

A `major` release will only be done via a governance gated upgrade. It can contain state breaking changes, and will
also generally include new features, major changes to existing features, and/or large updates to key dependency
packages such as CometBFT or the Cosmos SDK.

A `minor` release may be executed via a governance gated upgrade, or via a coordinated upgrade on a predefined block
height. It will contain breaking changes that require a coordinated upgrade, but the scope of these changes is
limited to essential updates such as fixes for security vulnerabilities.

Each vulnerability which requires a state breaking upgrade will be evaluated individually by the maintainers of the
software and the maintainers will determine on whether to include the changes into a minor release.

A `patch` release will be created for changes that are strictly not state breaking. The latest patch release for a
given release version is generally the recommended release, however, validator updates can be rolled out
asynchronously without risking the state of a network running the software.

The intention of the Release Policy is to ensure that the latest Babylond release is maintained with the following
categories of fixes:

- Tooling improvements (including code formatting, linting, static analysis and updates to testing frameworks)
- Performance enhancements for running archival and syncing nodes
- Test and benchmarking suites, ensuring that fixes are sound and there are no performance regressions
- Library updates including point releases for core libraries such as IBC-Go, Cosmos SDK, CometBFT and other dependencies
- General maintenance improvements, that are deemed necessary by the stewarding team, that help align different releases and reduce the workload on the stewarding team
- Security fixes

## Non-major Release Procedure

Updates to the release branch should come from `main` by backporting PRs
(usually done by automatic cherry-pick followed by PRs to the release branch).
The backports must be marked using `backport-to-release/vY.x` label in PR for main.
It is the PR author's responsibility to fix merge conflicts, update changelog entries, and
ensure CI passes. If a PR originates from an external contributor, a member of the stewarding team assumes
responsibility to perform this process instead of the original author.

After the release branch has all commits required for the next patch release:

* Update the [changelog](#changelog)
* Create a new annotated git tag in the release branch (follow the [Tagging Procedure](#tagging-procedure)).
* Once the release process completes, back port changelog updates to `main` branch
