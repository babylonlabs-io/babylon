<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should have following format:

* [#PullRequestNumber](PullRequestLink) message

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState
given same genesisState and txList.
Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

## [Unreleased](https://github.com/babylonchain/babylon-private/tree/HEAD)

[Full Changelog](https://github.com/babylonchain/babylon-private/compare/euphrates-0.2.0-rc.0...HEAD)


## [euphrates-v0.2.0-rc.0](https://github.com/babylonchain/babylon-private/tree/euphrates-v0.2.0-rc.0) (2024-05-17)

[Full Changelog](https://github.com/babylonchain/babylon-private/compare/euphrates-0.1.0-rc.1...euphrates-v0.2.0-rc.0)

## [euphrates-0.1.0-rc.1](https://github.com/babylonchain/babylon-private/tree/euphrates-0.1.0-rc.1) (2024-03-25)

[Full Changelog](https://github.com/babylonchain/babylon-private/compare/euphrates-0.1.0-rc.0...euphrates-0.1.0-rc.1)

**Fixed bugs:**

- Fix: only calculating Babylon FPs for FP set rotation (#223)

## [euphrates-0.1.0-rc.0](https://github.com/babylonchain/babylon-private/tree/euphrates-0.1.0-rc.0) (2024-03-22)

[Full Changelog](https://github.com/babylonchain/babylon-private/compare/v0.8.5...euphrates-0.1.0-rc.0)

**Closed issues:**

- handler for registering consumer chain finality providers [\#211](https://github.com/babylonchain/babylon-private/issues/211)
- restaking support and tests [\#208](https://github.com/babylonchain/babylon-private/issues/208)
- New module for BTC staking integration [\#204](https://github.com/babylonchain/babylon-private/issues/204)
- Consumer chain finality provider registry [\#203](https://github.com/babylonchain/babylon-private/issues/203)

## [v0.8.0](https://github.com/babylonchain/babylon/tree/v0.8.0) (2024-02-08)

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## Unreleased

### State Machine Breaking

* [#45](https://github.com/babylonlabs-io/babylon/pull/45) Implement ADR-23 and improve
BTC staking parameters
* [#51](https://github.com/babylonlabs-io/babylon/pull/51) Implement ADR-24 and
enable timestamping of the public randomness committed by finality providers
* [#32](https://github.com/babylonlabs-io/babylon/pull/32) Replace `chain-id`
with `client-id` to identify consumer chains in `zoneconcierge` module
* [#35](https://github.com/babylonlabs-io/babylon/pull/35) Add upgrade that
processes `MsgCreateFinalityProvider` message during upgrade execution
* [#4](https://github.com/babylonlabs-io/babylon/pull/4) Add upgrade that
Insert BTC headers into `btclightclient` module state during upgrade execution

## v0.9.3

## v0.9.2

### Improvements

* [#19](https://github.com/babylonlabs-io/babylon/pull/19) Modify Babylon node
LICENSE.md file

## v0.9.1

### Bug Fixes

* [#13](https://github.com/babylonlabs-io/babylon/pull/13) fix shadowing bug in
Babylon retry library

## v0.9.0

Initial Release!
