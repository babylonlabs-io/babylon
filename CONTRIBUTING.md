# Contributing

- [Contributing](#contributing)
  - [Overview](#overview)
    - [Pull request rules](#pull-request-rules)
  - [Development Procedure](#development-procedure)
    - [Testing](#testing)
    - [Pull Requests](#pull-requests)
    - [Requesting Reviews](#requesting-reviews)
    - [Updating Documentation](#updating-documentation)
    - [Updating Changelog](#updating-changelog)
  - [Dependencies](#dependencies)
  - [Protobuf](#protobuf)
  - [Branching Model and Release](#branching-model-and-release)
    - [PR Targeting](#pr-targeting)

## Overview

This document codifies rules that must be followed when contributing to
the Babylon node repository.

### Pull request rules

Every pull request must be easy to review. To make it possible:

 1. **Each pull request must do _one thing_**. It must be very clear what that
    one thing is when looking at the pull request title, description, and linked
    issues. It must also be very clear what value it ultimately aims to deliver,
    and for which user(s).

 2. **Each pull request must be manageable in size**. Self-contained pull
    requests that are manageable in size may target `main` directly. Larger
    contributions though must be structured as a series of smaller pull requests
    each building upon the previous one, all ideally tracked in a tracking issue
    These pull requests must target a long-lived feature branch. For details,
    see the [development procedure guidelines](#development-procedure).

    **Note**: This does not necessarily apply to documentation-related changes
    or automatically generated code (e.g. generated from Protobuf definitions).
    But automatically generated code changes should occur within separate
    commits, so they are easily distinguishable from manual code changes.

## Development Procedure

`main` must be stable, include only completed features and never fail `make
test`, `make test-e2e`, or `make build/install`.

Depending on the scope of the work, we differentiate between self-contained pull
requests and long-lived contributions (features).

All pr merges, either to `main` branch or to feature branch must be done by
squash and merge method.

**Self-contained pull requests**:

* Fork the repo (core developers must create a branch directly in the Babylon
repo), branch from the HEAD of `main`, make some commits, and submit a PR to
`main`.
* For developers who are core contributors and are working within the Babylon
repo, follow branch name conventions to ensure clear ownership of branches:
`{moniker}/branch-name`.
* See [Branching Model](#branching-model-and-release) for more details.

**Large contributions**:

* Make sure that a feature branch is created in the repo or create one. The name
  convention for the feature branch must be `feat/branch-name`. Note that
  (similar to `main`) all feature branches have branch protection rules and they
  run the CI. Unlike `main`, feature branch may intermittently fail `make test`,
  `make test-e2e`, or `make build/install`.
* Fork the repo (core developers must create a branch directly in the Babylon
  repo), branch from the HEAD of the feature branch, make some commits, and
  submit a PR to the feature branch. All PRs targeting a feature branch should
  follow the same guidelines in this document.
* Once the feature is completed, submit a PR from the feature branch targeting
  `main`.

### Testing

Tests can be executed by running `make test` at the top level of the Babylon
repository. Running e2e test can be accomplished by running `make test-e2e`

### Pull Requests

Before submitting a pull request:

* synchronize your branch with the latest base branch (i.e., `main` or feature
  branch) and resolve any arising conflicts, e.g.,
  - either `git fetch origin/main && git merge origin/main`
  - or `git fetch origin/main && git rebase -i origin/main`
* run `make test`, `make test-e2e`, `make build/install` to ensure that all
  checks and tests pass.

Then:

1. If you have something to show, **start with a `Draft` PR**. It's good to have
   early validation of your work and we highly recommend this practice. A Draft
   PR also indicates to the community that the work is in progress.
2. When the code is complete, change your PR from `Draft` to `Ready for Review`.

PRs must have a category prefix that is based on the type of changes being made
(for example, `fix`, `feat`, `refactor`, `docs`, and so on). The
[type](https://github.com/commitizen/conventional-commit-types/blob/v3.0.0/index.json)
must be included in the PR title as a prefix (for example, `fix:
<description>`). This convention ensures that all changes that are committed to
the base branch follow the [Conventional
Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification.

### Requesting Reviews

If you would like to receive early feedback on the PR, open the PR as a "Draft"
and leave a comment in the PR indicating that you would like early feedback and
tagging whoever you would like to receive feedback from.

All PRs require at least two review approvals before they can be merged (one
review might be acceptable in the case of minor changes or changes that do not
affect production code).

### Updating Documentation

If you open a PR in Babylon, it is mandatory to update the relevant
documentation in `/docs`.

### Updating Changelog

Any PR which is merged to `main` and affects consumers of the codebase,
must modify changelog [file](./CHANGELOG.md) accordingly, by adding new entry
in `Unreleased` section of the changelog.

Examples of changes which require change log update:
- bug fixes
- adding new APIs
- adding new features

Examples of changes that do not require changelog updates:
- refactoring of internal implementation
- adding new tests
- modifying documentation

The rule of thumb here is that the changelog should be updated as long as the change is
visible by external users.

## Dependencies

We use [Go Modules](https://github.com/golang/go/wiki/Modules) to manage
dependency versions.

The main branch of every Babylon repository should just build with `go get`,
which means they should be kept up-to-date with their dependencies so we can get
away with telling people they can just `go get` our software.

When dependencies in Babylon `go.mod` are changed, it is generally accepted
practice to delete `go.sum` and then run `go mod tidy`.

Since some dependencies are not under our control, a third party may break our
build, in which case we can fall back on `go mod tidy -v`.

## Protobuf

We use [Protocol Buffers](https://developers.google.com/protocol-buffers) along
with [gogoproto](https://github.com/cosmos/gogoproto) to generate code for use
in Babylon.

For deterministic behavior around Protobuf tooling, everything is containerized
using Docker. Make sure to have Docker installed on your machine, or head to
[Docker's website](https://docs.docker.com/get-docker/) to install it.

To generate the protobuf stubs, you can run `make proto-gen`.

## Branching Model and Release

User-facing repos should adhere to the trunk based development branching model:
https://trunkbaseddevelopment.com. User branches should start with a user name,
example: `{moniker}/branch-name`.

Babylon follows [semantic versioning](https://semver.org), but with the some
deviations to account for state-machine and API breaking changes. See
[RELEASE_PROCESS.md](./RELEASE_PROCESS.md) for details.

### PR Targeting

Ensure that you base and target your PRs on either `main` or a feature branch.

All complete features and bug fixes must be targeted against `main`.

Exception is for bug fixes which are only related to a released version. In that
case:
- either, bug fix must be targeted at `main` branch and later back ported to
  `release/` branch
- or if `main` and `release/` branched diverged too much, the fix can be
targeted to `release/` branch directly
