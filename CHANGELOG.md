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

All notable changes to this project will be documented in this file.

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
