# Release Process

- [Breaking Changes](#breaking-changes)
- [Release Procedure](#release-procedure)
  - [Creating a new release branch](#creating-a-new-release-branch)
  - [Cutting a new release](#cutting-a-new-release)
  - [Tagging Procedure](#tagging-procedure)
- [Patch Release Procedure](#patch-release-procedure)

This document outlines the release process for the Babylon node (babylond)

Babylon follows [semantic versioning](https://semver.org), but with the
following deviations to account for state-machine and API breaking changes.

- State-machine breaking changes & API breaking changes will result in an
increase of the minor version Y (0.Y.z).
- All other changes will result in an increase of the patch version Z (0.y.Z).

## Breaking Changes

A change is considered to be ***state-machine breaking*** if it requires a
coordinated upgrade for the network to preserve state compatibility Note that
when bumping the dependencies of [Cosmos
SDK](https://github.com/cosmos/cosmos-sdk) and
[IBC](https://github.com/cosmos/ibc-go), we will only treat patch releases as
non state-machine breaking.

A change is considered to be ***API breaking*** if it modifies the provided API.
This includes events, queries, CLI interfaces.

## Release Procedure

A _release_ is an increment of the second number (eg: `v0.1.0` → `v0.2.0`)

**Note**: Generally, PRs should target either `main` or a long-lived feature
branch (see [CONTRIBUTING.md](./CONTRIBUTING.md#pull-requests)).

* Once the team feels that `main` is _**feature complete**_, we create a
  `release/v0.Y.x` branch (going forward known as release branch), where `Y` is
  the minor version number, with patch part substituted to `x` (eg: v0.11.x).
  * **PRs targeting directly a release branch can be merged _only_ when
    exceptional circumstances arise**.
* In the release branch
  * Create a PR that adds new version section in the `CHANGELOG.md`, matching the released version e.g
    for branch `release/v0.Y.x`, section will be called `v0.Y.0`
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
    * Moves all changelog entries form `Unreleased` section of the changelog to the newly created section `v0.Y.0`
  * Create new release candidate tags after bugs are fixed.
* After the team feels the release candidate is ready, create a full release:
  * **Note:** The final release MUST have the same commit hash as the latest
    corresponding release candidate.
  * Create a new annotated git tag in the release branch (follow the [Tagging
    Procedure](#tagging-procedure))
* After the final release is made e.g `v0.Y.0`, backport changelog changes to the `main` branch
  * checkout a new branch from the main branch: `username/backport_changelog`
  * bring the new section from `release/v0.Y.x` branch to the `CHANGELOG.md` file on `main` branch
  * open PR against the `main` branch

### Creating a new release branch

- create a new release branch, e.g., `release/v0.10.x`
    ```bash
    git checkout main
    git pull
    git checkout -b release/v0.10.x
    ```
- push the release branch upstream
    ```bash
    git push
    ```
### Cutting a new release

Before cutting a release (e.g., `v0.10.0-rc.0`), the
following steps are necessary:

- move to the release branch, e.g., `release/v0.10.x`
    ```bash
    git checkout release/v0.10.x
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
you want to release version `v0.10.0-rc.0` ):
```bash
git pull --tags

git tag -s -a v0.10.0-rc.0 -m "Version v0.10.0-rc.0"
```

With every tag, the Github action will use the `goreleaser` tool to create a
release, including artifacts and their checksums.

## Patch Release Procedure

A _patch release_ is an increment of the patch number (eg: `v10.0.0` → `v10.0.1`).

**Important**: _**Patch releases can break consensus only in exceptional
circumstances .**_

Updates to the release branch should come from `main` by backporting PRs
(usually done by automatic cherry pick followed by a PR to the release branch).
The backports must be marked using `backport release/v0.Y.x` label in PR for
`main`, where `release/v0.Y.x` is the name of the release branch. It is the PR
author's responsibility to fix merge conflicts, update changelog entries, and
ensure CI passes. If a PR originates from an external contributor, a member of
the stewarding team assumes responsibility to perform this process instead of
the original author.

After the release branch has all commits required for the next patch release:
* Create a new annotated git tag in the release
branch (follow the [Tagging Procedure](#tagging-procedure)).
