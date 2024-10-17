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

## v0.13.0

### API Breaking

* [#194](https://github.com/babylonlabs-io/babylon/pull/194) Adjusted handling of `FinalityProviderSigningInfo` in finality keeper queries to improve API security.
  * Modified `QuerySigningInfosResponse` to remove direct exposure of sensitive fields.
  * Updated related tests in `x/finality/keeper/grpc_query_test.go`.
* [#200](https://github.com/babylonlabs-io/babylon/pull/200) Adjusted handling of `Gauge` in incentive keeper queries to improve API security.
* [#201](https://github.com/babylonlabs-io/babylon/pull/201) Adjusted handling of `ValidatorWithBlsKey` in checkpoint keeper queries to improve API security.
* [#202](https://github.com/babylonlabs-io/babylon/pull/202) Adjusted handling of `FinalityProviderWithMeta` in btcstaking keeper queries to improve API security.
* [#203](https://github.com/babylonlabs-io/babylon/pull/203) Adjusted handling of `RewardGauge` in incentive keeper queries to improve API security.

### State Machine Breaking

* [#181](https://github.com/babylonlabs-io/babylon/pull/181) Modify BTC heights
  and depths to be of uint32 type instead of uint64.

### Bug fixes

* [#197](https://github.com/babylonlabs-io/babylon/pull/197) Fix `BTCDelgationResponse` missing `staking_time`
* [#193](https://github.com/babylonlabs-io/babylon/pull/193) Fix witness construction of slashing tx
* [#154](https://github.com/babylonlabs-io/babylon/pull/154) Fix "edit-finality-provider" cmd argument index
* [#186](https://github.com/babylonlabs-io/babylon/pull/186) Do not panic on `nil`
Proof when handling finality votes

### Improvements

* [#188](https://github.com/babylonlabs-io/babylon/pull/188) Simplify logic of FP set rotation
* [#169](https://github.com/babylonlabs-io/babylon/pull/169) Improve external events format and update events doc
* [#148](https://github.com/babylonlabs-io/babylon/pull/148) Add block results query

### Misc Improvements

* [#170](https://github.com/babylonlabs-io/babylon/pull/170) Go releaser setup
* [#168](https://github.com/babylonlabs-io/babylon/pull/168) Remove devdoc from
  Makefile and remove unnecessary gin replace.
* [#184](https://github.com/babylonlabs-io/babylon/pull/184) Remove localnet
  setup as it provides no additional testing value.

## v0.12.1

### Bug fixes

* [#180](https://github.com/babylonlabs-io/babylon/pull/180) Non-determinism in
  sorting finality providers in the voting power table

### Improvements

* [#169](https://github.com/babylonlabs-io/babylon/pull/169) Improve external events format and update events doc

### State Machine Breaking

* [#185](https://github.com/babylonlabs-io/babylon/pull/185) Check that
unbonding / slashing transactions are standard

## v0.12.0

### State Machine Breaking

* [#132](https://github.com/babylonlabs-io/babylon/pull/132) Add CosmWasm parameters
update during v1 upgrade handler.
* [#142](https://github.com/babylonlabs-io/babylon/pull/142) Remove signed finality providers
insert from the v1 upgrade handler.

### Improvements

* [#151](https://github.com/babylonlabs-io/babylon/pull/151) Improve IBC transfer e2e test
* [#130](https://github.com/babylonlabs-io/babylon/pull/130) Fix bugs in the
transaction fee refunding mechanism for covenant signatures and finality signatures
* [#125](https://github.com/babylonlabs-io/babylon/pull/125) Implement ADR-028 and
refund transaction fee for certain transactions from protocol stakeholders
* [#137](https://github.com/babylonlabs-io/babylon/pull/137) Adapt tests to the
pre-approval flow.
* [#138](https://github.com/babylonlabs-io/babylon/pull/138) Intercept staking module
messages inside `authz.MsgExec`.
* [#146](https://github.com/babylonlabs-io/babylon/pull/146) Add property status as a filter
to BTC delegations rest request `QueryBTCDelegationsRequest`.
* [#144](https://github.com/babylonlabs-io/babylon/pull/144) Add new finality provider events
* [#131](https://github.com/babylonlabs-io/babylon/pull/131) Add new staking events
* [#113](https://github.com/babylonlabs-io/babylon/pull/113) Add multibuild binary
for upgrade handler `testnet` and `mainnet`.

### Bug Fixes
* [#141](https://github.com/babylonlabs-io/babylon/pull/141) Generate voting
power events only once when reaching covenant committee quorum
* [#140](https://github.com/babylonlabs-io/babylon/pull/140) Removed `unbonding`
and add `verified` to delegation status parse `NewBTCDelegationStatusFromString`.

## v0.11.0

### State Machine Breaking

* [#107](https://github.com/babylonlabs-io/babylon/pull/107) Implement ADR-027 and
enable in-protocol minimum gas price
* [#103](https://github.com/babylonlabs-io/babylon/pull/103) Add token distribution
to upgrade handler and rename `signet-launch` to `v1`
* [#55](https://github.com/babylonlabs-io/babylon/pull/55) Remove `x/zoneconcierge`
module
* [#117](https://github.com/babylonlabs-io/babylon/pull/117) Add support for
pre-approval flow (ADR-026)

### Bug fixes

### Misc Improvements

* [#106](https://github.com/babylonlabs-io/babylon/pull/106) Add CLI command for
  querying signing info of finality providers.

## v0.10.1

### Bug Fixes

* [#91](https://github.com/babylonlabs-io/babylon/pull/91) fix testnet command
by add ibc default gen state and min gas price specification of `1ubbn`
* [#93](https://github.com/babylonlabs-io/babylon/pull/93) fix genesis epoch
  initialization.

## v0.10.0

### State Machine Breaking

* [#80](https://github.com/babylonlabs-io/babylon/pull/80) Implement ADR-25 and
enable jailing/unjailing finality providers
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

### Misc Improvements

* [#84](https://github.com/babylonlabs-io/babylon/pull/84) Add `unjail-finality-provider`
cmd to `finality` module CLI.

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
