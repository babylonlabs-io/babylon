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

### State breaking

- [#1867](https://github.com/babylonlabs-io/babylon/pull/1867) bump wasmd `v0.60.2`
- [#1883](https://github.com/babylonlabs-io/babylon/pull/1883) Validate staker btc pk is the same with the previous btc del when stake extension
- [#1902](https://github.com/babylonlabs-io/babylon/pull/1902) fix: sorting of valset by the full address
- [#1826](https://github.com/babylonlabs-io/babylon/pull/1826) Support M-of-N multisig btc staker

### Bug Fixes

- [#1875](https://github.com/babylonlabs-io/babylon/pull/1875) chore: ensure soft-deleted FPs cannot receive new/extended BTC stake, or commit pub rand

### Improvements

- [#1878](https://github.com/babylonlabs-io/babylon/pull/1878) Remove unnecessary []byte allocations in IBC size checks
- [#1891](https://github.com/babylonlabs-io/babylon/pull/1891) fix: golangci lint misspell and removed unused func
- [#1901](https://github.com/babylonlabs-io/babylon/pull/1901) chore: update cl

## v4.2.2

### Improvements

- [#1903](https://github.com/babylonlabs-io/babylon/pull/1903) chore: bump cometbft to `v0.38.20`

## v4.2.1

### Improvements

- [#1839](https://github.com/babylonlabs-io/babylon/pull/1839) Add query to get the voting power distribution

## v4.2.0

### Bug Fixes

- [GHSA-m6wq-66p2-c8pc](https://github.com/babylonlabs-io/babylon/security/advisories/GHSA-m6wq-66p2-c8pc) fix: nil check of block hash in vote extension
- [GHSA-4rmq-mc2c-r495](https://github.com/babylonlabs-io/babylon-ghsa-4rmq-mc2c-r495/pull/1) Fix conditional logic in `AfterBtcDelegationUnbonded` hook

## v4.1.0

### Improvements

- [#1764](https://github.com/babylonlabs-io/babylon/pull/1764) Add mergify yaml file for automatic backporting

### Bug fixes

- [GHSA-2fcv-qww3-9v6h](https://github.com/babylonlabs-io/babylon-ghsa-2fcv-qww3-9v6h/pull/1) Add unkown fields check on vote extension validation
- [GHSA-xq4h-wqm2-668w](https://github.com/babylonlabs-io/babylon-ghsa-xq4h-wqm2-668w/pull/2) crypto: ensure BIP-322 signatures are using `SIGHASH_ALL` or `SIGHASH_DEFAULT`

## v4.0.0

## v4.0.0-rc.3

### Improvements

- [#1848](https://github.com/babylonlabs-io/babylon/pull/1848) Add testnet v4rc3 upgrade handler

## v4.0.0-rc.2

### State breaking

- [#1832](https://github.com/babylonlabs-io/babylon/pull/1832) Add check for multiple txOut with same pk and
value in `GetOutputIdxInBTCTx`
- [#1841](https://github.com/babylonlabs-io/babylon/pull/1841) Add validation for max concurrent unbonding and
redelegation in `MsgWrappedUndelegate` and `MsgWrappedBeginRedelegate`.
- [#1842](https://github.com/babylonlabs-io/babylon/pull/1842) Add `MsgWrappedCreateValidator` queue msg details

### Improvements

- [#1831](https://github.com/babylonlabs-io/babylon/pull/1831) Removed unused parameter in `DeriveTaprootPkScript`

## v4.0.0-rc.1

### Improvements

- [#1762](https://github.com/babylonlabs-io/babylon/pull/1762) Add new test case on stake expansion
- [#1785](https://github.com/babylonlabs-io/babylon/pull/1785) CI reusable to v0.13.5 and golang lint version 2
- [#1776](https://github.com/babylonlabs-io/babylon/pull/1776) Track baby staked to active validators only
- [#1813](https://github.com/babylonlabs-io/babylon/pull/1813) Panic if there is an invalid amount of sats in the
finality provider distribution info

### Bug fixes

- [#1763](https://github.com/babylonlabs-io/babylon/pull/1763) Fix `v4` upgrade logic to store co-staker reward tracker for all stakers (baby and/or BTC)
- [#1771](https://github.com/babylonlabs-io/babylon/pull/1771) Fix `x/costaking` costakers portion to 2.35% of 5.5%
- [#1780](https://github.com/babylonlabs-io/babylon/pull/1780) Fix `x/costaking` `MsgUpdateParams` update score to
baby ratio where it wasn't increasing the costaker cumulative rewards and neither sending the previous rewards to gauge.
- [#1783](https://github.com/babylonlabs-io/babylon/pull/1783) Fix `x/costaking` calls baby delegation modified in hook `BeforeDelegationRemoved`
as the completely removal of an baby delegation doesn't calls `AfterDelegationModified
- [#1790](https://github.com/babylonlabs-io/babylon/pull/1790) Fix withdraw reward to only error if both `BTC_STAKER` and `COSTAKER` types have zero rewards available.
- [#1792](https://github.com/babylonlabs-io/babylon/pull/1792) Fix costaking baby bond unbond and bond again for the same delegation pair (del, val) in the same block
- [#1800](https://github.com/babylonlabs-io/babylon/pull/1800) Handle co-staking edge cases for baby stakers
- [#1802](https://github.com/babylonlabs-io/babylon/pull/1802) Fix non-determinism in co-staking
- [#1805](https://github.com/babylonlabs-io/babylon/pull/1805) Fix non-determinism in incentives `IterateBTCDelegationSatsUpdated`
- [#1815](https://github.com/babylonlabs-io/babylon/pull/1815) Fix export genesis with Costaker type in `x/incentive`

## v4.0.0-rc.0

### Improvements

- [#1734](https://github.com/babylonlabs-io/babylon/pull/1734) Bump repository version to `v4`
- [#1736](https://github.com/babylonlabs-io/babylon/pull/1736) bump cosmos-sdk to `v0.53.2`
- [#1633](https://github.com/babylonlabs-io/babylon/pull/1663) Add spam prevention mechanisms to epoching module with minimum amount validation and gas consumption limits for wrapped delegate operations.
- [#1404](https://github.com/babylonlabs-io/babylon/pull/1404) Refactor power dist event processing.
- [#1347](https://github.com/babylonlabs-io/babylon/pull/1344) Add `MsgType` field to `QueuedMessageResponse` message.
- [#1737](https://github.com/babylonlabs-io/babylon/pull/1737) bump ibc-go to `v10` and wire up IBC `v2`
- [#1747](https://github.com/babylonlabs-io/babylon/pull/1747) bump cosmos-sdk to `v0.53.4`
- [#1062](https://github.com/babylonlabs-io/babylon/pull/1062) Add `anteHandler` to avoid fee grants on refundable tx (`x/incentive`)
- [#1060](https://github.com/babylonlabs-io/babylon/pull/1060) Optimize `PubRandCommit` lookup in `x/finality` module
- [#1335](https://github.com/babylonlabs-io/babylon/pull/1335) Tweaks on BTC stake expansion feature.
- [#1352](https://github.com/babylonlabs-io/babylon/pull/1352) Add stake expansion signatures in BTC delegation query response
- [#1369](https://github.com/babylonlabs-io/babylon/pull/1369) fix: update stake expansion validation to allow expanding
- [#1418](https://github.com/babylonlabs-io/babylon/pull/1418) fix: add stake expansion covenant signature to the events
- [#1740](https://github.com/babylonlabs-io/babylon/pull/1740) Simplify selective slashing message handler
- [#1416](https://github.com/babylonlabs-io/babylon/pull/1416) Relax requirements for stake expansion funding output
- [#1428](https://github.com/babylonlabs-io/babylon/pull/1428) Update stake expansion events
- [#1545](https://github.com/babylonlabs-io/babylon/pull/1545) fix: remove unnecessary covenant verification for stake expansion delegation
- [#1410](https://github.com/babylonlabs-io/babylon/pull/1410) Add stake expansion e2e test
- [#1707](https://github.com/babylonlabs-io/babylon/pull/1707) Add `x/costaking`

### Features

- [#1231](https://github.com/babylonlabs-io/babylon/pull/1231) Add BTC stake expansion.

## v2.3.2

### Bug fixes

- [#1808](https://github.com/babylonlabs-io/babylon/pull/1808) Updates cometbft to v0.38.19 fix ASA-2025-003.

## v2.3.1

### Improvements

- [#1775](https://github.com/babylonlabs-io/babylon/pull/1775) testnet: add upgrade `v23` to testnet build tag
for bbn-test-6

## v2.3.0

### Improvements

- [#1269](https://github.com/babylonlabs-io/babylon/pull/1269) Add fee collector e2e tests.
- [#1344](https://github.com/babylonlabs-io/babylon/pull/1344) Bump cosmos-sdk and
remove send restrictions
- [#1398](https://github.com/babylonlabs-io/babylon/pull/1398) Add custom tokenfactory
bindings (thanks @benluelo for contribution)
- [#1594](https://github.com/babylonlabs-io/babylon/pull/1594) Add soft deleted to finality providers response.
- [#1611](https://github.com/babylonlabs-io/babylon/pull/1611) Rename `x/btcstaking` prefixes as `v3.x`.
- [#1668](https://github.com/babylonlabs-io/babylon/pull/1668) Bump dependencies based on dependabot alerts

### Bug Fixes

- [#1355](https://github.com/babylonlabs-io/babylon/pull/1355) Allow empty fees on genesis transactions
- [#1453](https://github.com/babylonlabs-io/babylon/pull/1453) fix: inactive FP event on full unbonding
- [#1593](https://github.com/babylonlabs-io/babylon/pull/1593) fix: disabled duplicate fp bbn addr registering
- [#1714](https://github.com/babylonlabs-io/babylon/pull/1714) Add BLS key flags to `rollback` and `bootstrap-state` cmds

## v2.2.0

### Improvements

- [GHSA-56j4-446m-qrf6](https://github.com/babylonlabs-io/babylon-ghsa-56j4-446m-qrf6/pull/1) Add bank restriction for fee collector account

## v2.1.0

- [#1244](https://github.com/babylonlabs-io/babylon/pull/1244) fix: remove staking module from app end blocker order

## v2.0.0

- [#1226](https://github.com/babylonlabs-io/babylon/pull/1226) chore: Revert goreleaser to use musl

## v2.0.0-rc.4

### Improvements

- [#1191](https://github.com/babylonlabs-io/babylon/pull/1191) fix: update fp commission
- [#1109](https://github.com/babylonlabs-io/babylon/pull/1190) Use glibc for goreleaser
- [#1197](https://github.com/babylonlabs-io/babylon/pull/1197) fix: add validate of bad unbonding fee in btcstaking params.
- [#1211](https://github.com/babylonlabs-io/babylon/pull/1211) Update upgrade handler name to v2rc4

### State Machine Breaking

- [#1207](https://github.com/babylonlabs-io/babylon/pull/1207) Update wasmd to [v0.54.1](https://github.com/CosmWasm/wasmd/releases/tag/v0.54.1)

### Bug fixes

- [#1196](https://github.com/babylonlabs-io/babylon/pull/1196) fix: add pagination in `QueryBlsPublicKeyListRequest`.

## v2.0.0-rc.3

### Improvements

- [#1065](https://github.com/babylonlabs-io/babylon/pull/1065) Add check for period of current rewards to be larger than zero.
- [#1128](https://github.com/babylonlabs-io/babylon/pull/1128) Validate genesis fp historic reward entries.
- [#1064](https://github.com/babylonlabs-io/babylon/pull/1064) Signing info validation for `StartHeight` and `MissedBlockCounter`.
- [#1034](https://github.com/babylonlabs-io/babylon/pull/1034) Remove redundant `ValidateBasic` call in `x/btcstaking` message server
- [#1036](https://github.com/babylonlabs-io/babylon/pull/1036) Remove token factory `enable_admin_sudo_mint` capability
- [#1046](https://github.com/babylonlabs-io/babylon/pull/1046) Update genesis & validations `x/btcstaking` module
- [#1050](https://github.com/babylonlabs-io/babylon/pull/1050) Add query to get btc delegations at specific block height
- [#1061](https://github.com/babylonlabs-io/babylon/pull/1061) Add size and hex decode in
`RefundableMsgHashes` validate genesis
- [#1071](https://github.com/babylonlabs-io/babylon/pull/1071) Update `FinalityProviderDistInfo` validations
- [#1069](https://github.com/babylonlabs-io/babylon/pull/1069) Add validation on the txkey in btcstaking
- [#1070](https://github.com/babylonlabs-io/babylon/pull/1070) fix: validation for vp dist cache
- [#1078](https://github.com/babylonlabs-io/babylon/pull/1078) fix: enforce check ibc msg size in `finalizeBlockState`
- [#1082](https://github.com/babylonlabs-io/babylon/pull/1082) chore: val sequential epoch
- [#1083](https://github.com/babylonlabs-io/babylon/pull/1083) Check if `WithdrawAddress` is a blocked address in `SetWithdrawAddress`
- [#1084](https://github.com/babylonlabs-io/babylon/pull/1084) Check for negative amount in `TotalActiveSat` in `subFinalityProviderStaked`
- [#1085](https://github.com/babylonlabs-io/babylon/pull/1085) Update comment in `SetRewardTrackerEvent` function
- [#1089](https://github.com/babylonlabs-io/babylon/pull/1089) chore: validate block diff
- [#1096](https://github.com/babylonlabs-io/babylon/pull/1096) chore: validate stats positive
- [#1097](https://github.com/babylonlabs-io/babylon/pull/1097) Update `Evidence.ValidateBasic` function
- [#1098](https://github.com/babylonlabs-io/babylon/pull/1098) Support `HasValidateBasic` interface in `ValidateEntries` function
- [#1102](https://github.com/babylonlabs-io/babylon/pull/1102) Handle empty `BTCStakingGauge` when no fees are intercepted
- [#1118](https://github.com/babylonlabs-io/babylon/pull/1118) Update `RawCheckpointWithMeta` and `BlsMultiSig` validations
- [#1126](https://github.com/babylonlabs-io/babylon/pull/1126) chore: add bls key validation
- [#1135](https://github.com/babylonlabs-io/babylon/pull/1135) Add validation to `LastProcessedHeightEventRewardTracker` on `InitGenesis`
- [#1136](https://github.com/babylonlabs-io/babylon/pull/1136) Fix `SubmissionEntry` duplicate validation in `InitGenesis`
- [#1147](https://github.com/babylonlabs-io/babylon/pull/1147) chore: vp dist cache count active fps
- [#1151](https://github.com/babylonlabs-io/babylon/pull/1151) Add whitelisted channels to add rate limit in `v2` upgrade.
- [#1152](https://github.com/babylonlabs-io/babylon/pull/1152) chore: validate power non negative.
- [#1168](https://github.com/babylonlabs-io/babylon/pull/1168) chore: removed duplicated addr len check in `SetAddressVerifier`.
- [#1171](https://github.com/babylonlabs-io/babylon/pull/1171) chore: add validation of `DelegationLifecycle`.
- [#1174](https://github.com/babylonlabs-io/babylon/pull/1174) chore: reduced ibc `MaxAddressSize` to max value of bech 32 addr (90).
- [#1181](https://github.com/babylonlabs-io/babylon/pull/1181) Update upgrade handler name to v2rc3

### State Machine Breaking

- [#1146](https://github.com/babylonlabs-io/babylon/pull/1146) Remove deprecated `x/crisis` module.

## v2.0.0-rc.2

### Improvements

- [#981](https://github.com/babylonlabs-io/babylon/pull/981) Set IBC rate limit for `ubbn` to 10%
- [#1013](https://github.com/babylonlabs-io/babylon/pull/1013) Update to ibc rate limit `v8.1.0`
- [#1018](https://github.com/babylonlabs-io/babylon/pull/1018) Improve genesis validations in `x/mint` and `x/btccheckpoint`
- [#1030](https://github.com/babylonlabs-io/babylon/pull/1030) Update upgrade handler name to v2rc2

## v2.0.0-rc.1

### Improvements

- [#931](https://github.com/babylonlabs-io/babylon/pull/931) Add darwin and linux-arm64 build
- [#992](https://github.com/babylonlabs-io/babylon/pull/992) Add checks for bad halting height in `HandleResumeFinalityProposal`
- [#993](https://github.com/babylonlabs-io/babylon/pull/993) Remove async-icq
  module.

### State Machine Breaking

- [#947](https://github.com/babylonlabs-io/babylon/pull/947) Improve signing info update upon fp activated
- [#994](https://github.com/babylonlabs-io/babylon/pull/994) Soft fork ibcratelimit to call `BeginBlock`

## v2.0.0-rc.0

### Improvements

- [#850](https://github.com/babylonlabs-io/babylon/pull/850) Add BLS improvement including permission and verification
- [#841](https://github.com/babylonlabs-io/babylon/pull/841) chore: upgrade the make file: release
- [#909](https://github.com/babylonlabs-io/babylon/pull/909) Update tokenfactory upgrade fee params to `ubbn`.
- [#884](https://github.com/babylonlabs-io/babylon/pull/884) Bump tokenfactory version to v0.50.6
- [#846](https://github.com/babylonlabs-io/babylon/pull/846) Add tokenfactory module.
- [#821](https://github.com/babylonlabs-io/babylon/pull/821) Add import/export genesis logic in `x/btccheckpoint` module
- [#828](https://github.com/babylonlabs-io/babylon/pull/828) Add `AllowedStakingTxHashes` and `LargetsBTCReorg` to import/export genesis logic in `x/btcstaking` module
- [#840](https://github.com/babylonlabs-io/babylon/pull/840) Add import/export genesis logic in `x/checkpointing` module
- [#843](https://github.com/babylonlabs-io/babylon/pull/843) Add import/export genesis logic in `x/epoching` module
- [#847](https://github.com/babylonlabs-io/babylon/pull/847) Add ibc callbacks to
transfer stack
- [#859](https://github.com/babylonlabs-io/babylon/pull/859) Update init/export genesis logic in `x/finality` module
- [#876](https://github.com/babylonlabs-io/babylon/pull/876) Add Packet Forwarding Middleware (PFM) module.
- [#881](https://github.com/babylonlabs-io/babylon/pull/881) Update init/export genesis logic in `x/mint` module
- [#883](https://github.com/babylonlabs-io/babylon/pull/883) Update init/export genesis logic in `x/monitor` module
- [#897](https://github.com/babylonlabs-io/babylon/pull/897) Add Interchain Accounts (ICA) and Interchain Queries (ICQ) modules.
- [#902](https://github.com/babylonlabs-io/babylon/pull/902) Add IBC rate limiter to IBC transfer module
- [#913](https://github.com/babylonlabs-io/babylon/pull/913) Remove IBC-fee module.

### State Machine Breaking

- [#877](https://github.com/babylonlabs-io/babylon/pull/877) fix: BTC reward tracker are stored as events and updated only
when the babylon block is being BTC rewarded.

## v1.1.0

### Bug fixes

- [#860](https://github.com/babylonlabs-io/babylon/pull/860) fix: ensure active fps have signing info after finality resumption proposal.
- [#842](https://github.com/babylonlabs-io/babylon/pull/842) fix: Properly wire `btclightclient` hook for
`btcstaking` to panic in case of a BTC reorg larger than `BtcConfirmationDepth`.
- [#829](https://github.com/babylonlabs-io/babylon/pull/829) fix: add checks for slashed fp in gov resume finality.
- [#905](https://github.com/babylonlabs-io/babylon/pull/905) Add `ValidateBasic` for `CommitPubRandList`
- [#906](https://github.com/babylonlabs-io/babylon/pull/906) Add distribution bank wrapper.

## v1.0.3

### Improvements

- [#1769](https://github.com/babylonlabs-io/babylon/pull/1769) testnet: update `v1` upgrade data for bbn-test-6

## v1.0.2

### Improvements

- [#802](https://github.com/babylonlabs-io/babylon/pull/802) fix: clean up resources allocated by TmpAppOptions
- [#805](https://github.com/babylonlabs-io/babylon/pull/805) chore: upgrade the make file: linting
- [#816](https://github.com/babylonlabs-io/babylon/pull/816) chore: upgrade the make file: gosec

## v1.0.1

### Bug fixes

- [#793](https://github.com/babylonlabs-io/babylon/pull/793) fix: BLS key will be overwritten when the password is not retrieved

## v1.0.0

## v1.0.0-rc11

### Bug fixes

- [#780](https://github.com/babylonlabs-io/babylon/pull/780) crypto: remove Verify for ECDSA

## v1.0.0-rc10

### Improvements

- [#761](https://github.com/babylonlabs-io/babylon/pull/746) Add mainnet allowed transaction hash to v1 upgrade handler.
- [#760](https://github.com/babylonlabs-io/babylon/pull/760) Add mainnet BTC headers
height from `854785` to `890123`.
- [#757](https://github.com/babylonlabs-io/babylon/pull/757) Statically link wasm and add binaries
with [`testnet`, `mainnet`] flags to release assets.
- [#746](https://github.com/babylonlabs-io/babylon/pull/746) Add mainnet parameters to v1 upgrade handler.

## v1.0.0-rc9

### Improvements

- [#749](https://github.com/babylonlabs-io/babylon/pull/749) Upgrade handler for `v1rc9`

### State breaking

- [#745](https://github.com/babylonlabs-io/babylon/pull/745) Hard limit of the number of finalized
and rewarded blocks

### Bug fixes

- [#748](https://github.com/babylonlabs-io/babylon/pull/748) fix: gov resume finality, index block before tally.
- [#731](https://github.com/babylonlabs-io/babylon/pull/731) chore: fix block timeout in Babylon client

## v1.0.0-rc8

### API Breaking

- [#690](https://github.com/babylonlabs-io/babylon/pull/690) Add new BLS password flow which includes env variable.
- [#682](https://github.com/babylonlabs-io/babylon/pull/682) Avoid creating pop in `babylond tx checkpointing create-validator`

### Bug fixes

- [#657](https://github.com/babylonlabs-io/babylon/pull/657) crypto: fix adaptor sig timing side channels
- [#656](https://github.com/babylonlabs-io/babylon/pull/656) crypto: fix adaptor sig validity and typos
- [#658](https://github.com/babylonlabs-io/babylon/pull/658) crypto: check if Z==1 in ToBTCPK
- [#667](https://github.com/babylonlabs-io/babylon/pull/667) crypto: enable groupcheck in BLS verification/aggregation
- [#660](https://github.com/babylonlabs-io/babylon/pull/660) fix: ecdsa verification
- [#680](https://github.com/babylonlabs-io/babylon/pull/680) crypto: fix bls rogue attack
- [#673](https://github.com/babylonlabs-io/babylon/pull/673) fix: move bip322 signing functions to `testutil`
- [#683](https://github.com/babylonlabs-io/babylon/pull/683) crypto: fix eots signing timing attack
- [#691](https://github.com/babylonlabs-io/babylon/pull/691) crypto: fix eots missing normalization in use of secp256k1.FieldVal
- [#671](https://github.com/babylonlabs-io/babylon/pull/671) crypto: align adaptor sig impl with Blockstream spec
- [#705](https://github.com/babylonlabs-io/babylon/pull/705) Add bls key length validation from the ERC-2335 keystore
- [#712](https://github.com/babylonlabs-io/babylon/pull/712) fix: remove exponentially events emission at processing queued msgs at the end epoch.

### Improvements

- [#701](https://github.com/babylonlabs-io/babylon/pull/701) Update upgrade name to `v1rc8`
- [#687](https://github.com/babylonlabs-io/babylon/pull/687) Add details to btc-reorg runbook.
- [#655](https://github.com/babylonlabs-io/babylon/pull/655) Add func `ParseV0StakingTxWithoutTag` to
parse staking tx without verifying opreturn tag.
- [#666](https://github.com/babylonlabs-io/babylon/pull/666) Upgrade to wasmvm v2.2.3.
- [#668](https://github.com/babylonlabs-io/babylon/pull/668) Remove unused unsafe key gen functions
- [#676](https://github.com/babylonlabs-io/babylon/pull/676) Bump IBC-go to `v8.7.0`
- [#644](https://github.com/babylonlabs-io/babylon/pull/644) Add priority nonce mempool and transaction priority ante handler decorator
- [#693](https://github.com/babylonlabs-io/babylon/pull/693) chore: use timeout from config in bbn client
- [#660](https://github.com/babylonlabs-io/babylon/pull/660) add function to recover pub key from sig
- [#625](https://github.com/babylonlabs-io/babylon/pull/625) add tx gas limit decorator and local mempool config

### State breaking

- [#697](https://github.com/babylonlabs-io/babylon/pull/697) Update BIP322 PoP and
ECDSA Pop to sign bech32 encoded cosmos address
- [#695](https://github.com/babylonlabs-io/babylon/pull/695) Improve checkpoint panicking behavior

## v1.0.0-rc7

### Improvements

- [#648](https://github.com/babylonlabs-io/babylon/pull/648) Add query to get all parameters
from `x/btcstaking` module.
- [#544](https://github.com/babylonlabs-io/babylon/pull/544) Add `bls-config` to `app.toml` for custom bls key location.
- [#558](https://github.com/babylonlabs-io/babylon/pull/558) Change BLS public key format from hex to base64 in bls_key.json.
- [#466](https://github.com/babylonlabs-io/babylon/pull/466) Add e2e test to
- [#519](https://github.com/babylonlabs-io/babylon/pull/519) Add missing data in `InitGenesis` and `ExportGenesis` in `x/incentive` module
block bank send and still create BTC delegations
- [#538](https://github.com/babylonlabs-io/babylon/pull/538) Upgrade to wasmd v0.54.x and wasmvm v2.2.x
- [#527](https://github.com/babylonlabs-io/babylon/pull/527) Create BSL signer on start command with flags.
- [#554](https://github.com/babylonlabs-io/babylon/pull/554) Improve vote extension logs
- [#566](https://github.com/babylonlabs-io/babylon/pull/566) Remove float values in `BeforeValidatorSlashed` hook in `x/epoching` module
- [#542](https://github.com/babylonlabs-io/babylon/pull/542) Add missing data in `InitGenesis` and `ExportGenesis` in `x/incentive` module (follow up of [#519](https://github.com/babylonlabs-io/babylon/pull/519))
- [#589](https://github.com/babylonlabs-io/babylon/pull/589) Rename `btc_delegation` stakeholder type to `btc_staker` in `x/incentive` module
- [#590](https://github.com/babylonlabs-io/babylon/pull/590) Add `DelegationRewards` query in `x/incentive` module
- [#625](https://github.com/babylonlabs-io/babylon/pull/625) Add `GasLimit` decorator in ante handler and local mempool configuration
- [#633](https://github.com/babylonlabs-io/babylon/pull/633) Fix swagger

### State Machine Breaking

- [#518](https://github.com/babylonlabs-io/babylon/pull/518) Add check BTC reorg blocks higher than `k` deep
- [#530](https://github.com/babylonlabs-io/babylon/pull/530) Add `ConflictingCheckpointReceived` flag in `x/checkpointing` module.
- [#537](https://github.com/babylonlabs-io/babylon/pull/537) Add `CommissionRates` type to `MsgCreateFinalityProvider` and commission validation to `EditFinalityProvider` in `x/btcstaking` module
- [#567](https://github.com/babylonlabs-io/babylon/pull/567) Add check for height overflow in `CommitPubRandList` in `x/finality` module
- [#620](https://github.com/babylonlabs-io/babylon/pull/620) fix: Incorrect set of JailUntil after unjailing

### Bug fixes

- [#539](https://github.com/babylonlabs-io/babylon/pull/539) fix: add missing `x/checkpointing` hooks
invocation
- [#591](https://github.com/babylonlabs-io/babylon/pull/591) bump ibc to v8.6.1 that fixes security issue
- [#579](https://github.com/babylonlabs-io/babylon/pull/579) Slashed FP gets activated in voting power distribution
cache if an old BTC delegation receives inclusion proof
- [#592](https://github.com/babylonlabs-io/babylon/pull/592) finality: avoid refunding finality signatures over forks
- [#525](https://github.com/babylonlabs-io/babylon/pull/525) fix: add back `NewIBCHeaderDecorator` post handler
- [#563](https://github.com/babylonlabs-io/babylon/pull/563) reject coinbase staking transactions
- [#584](https://github.com/babylonlabs-io/babylon/pull/584) fix: Panic can be triggered in handling liveness
- [#585](https://github.com/babylonlabs-io/babylon/pull/585) fix: Proposal vote extensions' byte limit
- [#594](https://github.com/babylonlabs-io/babylon/pull/594) Refund tx to correct recipient
- [#599](https://github.com/babylonlabs-io/babylon/pull/599) check staker signature in `BTCUndelegate`
- [#631](https://github.com/babylonlabs-io/babylon/pull/631) Ignore expired events
if delegation was never activated
- [#629](https://github.com/babylonlabs-io/babylon/pull/629) Allow OP_RETURN as slashing output
- [#597](https://github.com/babylonlabs-io/babylon/pull/597) fix: Expired and Unbonding delegation
in the same BTC block could lead to a panic and chain halt
- [#647](https://github.com/babylonlabs-io/babylon/pull/647) Ignore unbonding event
if delegation did not have covenant quorum

## v1.0.0-rc6

### Improvements

- [#508](https://github.com/babylonlabs-io/babylon/pull/508) Move PoP constructor functiosn to datagen/
- [#499](https://github.com/babylonlabs-io/babylon/pull/499) Add `params-by-version` CLI command
- [#515](https://github.com/babylonlabs-io/babylon/pull/515) Add `staker_addr` to `EventBTCDelegationCreated`
- [#458](https://github.com/babylonlabs-io/babylon/pull/458) Set `CosmosProvider` functions as public in
`babylonclient` package

### Bug fixes

- [#509](https://github.com/babylonlabs-io/babylon/pull/509) crypto: fix ECDSA malleability
- [#486](https://github.com/babylonlabs-io/babylon/pull/486) crypto: blinding base mult of nonce
- [#443](https://github.com/babylonlabs-io/babylon/pull/443) Fix swagger generation for incentive missing `v1` in path
- [#505](https://github.com/babylonlabs-io/babylon/pull/505) Fix `x/btcstaking`
delegation queries

## v1.0.0-rc5

### Improvements

- [#483](https://github.com/babylonlabs-io/babylon/pull/483) Upgrade wasmd and wasmvm to latest
versions (related to security advisories CWA-2025-001 and CWA-2025-002)
- [#464](https://github.com/babylonlabs-io/babylon/pull/464) Update security email. Fix site / repository refs
- [#421](https://github.com/babylonlabs-io/babylon/pull/421) Add checks to public
randomness commit at `TestBTCRewardsDistribution`.
- [#445](https://github.com/babylonlabs-io/babylon/pull/445) Reject BTC headers
forks starting with already known header
- [#457](https://github.com/babylonlabs-io/babylon/pull/457)  Remove staking msg
server and update gentx to generate `MsgWrappedCreateValidator`
- [#473](https://github.com/babylonlabs-io/babylon/pull/473) Fix checkpoint status transition
from `Finalized` to `Forgotten`
- [#457](https://github.com/babylonlabs-io/babylon/pull/457) Remove staking msg server and update gentx to generate
  `MsgWrappedCreateValidator`
- [#476](https://github.com/babylonlabs-io/babylon/pull/476) Bump cometbft to `v0.38.17`
- [#491](https://github.com/babylonlabs-io/babylon/pull/491) Allow slashing rate
to have 4 decimal places
- [#488](https://github.com/babylonlabs-io/babylon/pull/488) Fix duplicate BLS key registration in testnet command
- [#493](https://github.com/babylonlabs-io/babylon/pull/493) Add v1rc5 testnet
upgrade handler

## v1.0.0-rc4

### Improvements

- [#419](https://github.com/babylonlabs-io/babylon/pull/419) Add new modules to swagger config
- [#429](https://github.com/babylonlabs-io/babylon/pull/429) chore: remove cosmos/relayer dependency

### Bug fixes

- [#353](https://github.com/babylonlabs-io/babylon/pull/353) Bump to SDK
  0.50.11
- [#404](https://github.com/babylonlabs-io/babylon/pull/404) Improve adaptor
signature nonce generation to match reference implementation
- [#413](https://github.com/babylonlabs-io/babylon/pull/413) Fix adaptor
signature R verification
- [#441](https://github.com/babylonlabs-io/babylon/pull/441) Fix fuzzing test for
`CreateBTCDelegationWithParamsFromBtcHeight`

## v1.0.0-rc3

### Bug fixes

- [#374](https://github.com/babylonlabs-io/babylon/pull/374) Fix non-consecutive finalization
of the block in `TallyBlocks` function
- [#378](https://github.com/babylonlabs-io/babylon/pull/378) Fix give out rewards
with gaps of unfinalized blocks
- [#385](https://github.com/babylonlabs-io/babylon/pull/385) Fix epoching module
ante handler to return from antehandler chain only in case of error

## v1.0.0-rc2

### Bug fixes

- [#371](https://github.com/babylonlabs-io/babylon/pull/371) Do not prune BTC
reward tracker structures at the slash of finality provider.

## v1.0.0-rc.1

### Improvements

- [#306](https://github.com/babylonlabs-io/babylon/pull/306) feat: improve BTC reward distribution with
virtual block periods for each finality provider that has delegations and reward tracker structures.
- [#338](https://github.com/babylonlabs-io/babylon/pull/338) Add print BIP-340 in
`debug pubkey-raw` subcommand
- [#316](https://github.com/babylonlabs-io/babylon/pull/316) Add testnet upgrade data
- [#326](https://github.com/babylonlabs-io/babylon/pull/326) docs: btcstaking:
Update btcstaking module docs to include EOI
- [#348](https://github.com/babylonlabs-io/babylon/pull/348) refactory `PrivateSigner`
and module account vars in appparams
- [#351](https://github.com/babylonlabs-io/babylon/pull/351) docs: Add state
transition docs.
- [#358](https://github.com/babylonlabs-io/babylon/pull/358) Remove unused deps in `.proto` files
- [#364](https://github.com/babylonlabs-io/babylon/pull/364) Add testnet upgrade data

### Bug fixes

- [#324](https://github.com/babylonlabs-io/babylon/pull/324) Fix decrementing
jailed fp counter
- [#352](https://github.com/babylonlabs-io/babylon/pull/352) Fix: withdrawal cli
for rewards

### State Machine Breaking

- [#341](https://github.com/babylonlabs-io/babylon/pull/341) Select parameters
for pre-approval flow based on BTC LC tip height
- [#360](https://github.com/babylonlabs-io/babylon/pull/360) Refactor rewarding
- [#365](https://github.com/babylonlabs-io/babylon/pull/365) Reject outdated finality votes

## v0.18.2

### Bug fixes

- [#342](https://github.com/babylonlabs-io/babylon/pull/342) Fix non-determinism while jailing

## v0.18.1

- [#328](https://github.com/babylonlabs-io/babylon/pull/328) Fix btc activation height validation in genesis

## v0.18.0

### Improvements

- [#309](https://github.com/babylonlabs-io/babylon/pull/309) feat(adr-036): custom withdrawal address
- [#305](https://github.com/babylonlabs-io/babylon/pull/305) chore: add more error logs to `VerifyInclusionProofAndGetHeight`
- [#304](https://github.com/babylonlabs-io/babylon/pull/304) Add highest voted height to finality provider
- [#314](https://github.com/babylonlabs-io/babylon/pull/314) Require exact unbonding time in delegation
- [#317](https://github.com/babylonlabs-io/babylon/pull/317) Enforce that EOI
delegations using correct parameters version

### State Machine Breaking

- [#310](https://github.com/babylonlabs-io/babylon/pull/310) implement adr-37 -
making params valid for btc light client ranges

### Bug fixes

- [#318](https://github.com/babylonlabs-io/babylon/pull/318) Fix BTC delegation status check
to relay on UnbondingTime in delegation

## v0.17.2

### Improvements

- [#311](https://github.com/babylonlabs-io/babylon/pull/311) Enforce version 2
for unbonding transactions

## v0.17.1

### Bug fixes

- [#289](https://github.com/babylonlabs-io/babylon/pull/289) hotfix: Invalid minUnbondingTime for verifying inclusion proof

## v0.17.0

### State Breaking

- [278](https://github.com/babylonlabs-io/babylon/pull/278) Allow unbonding time to be min unbonding value

### Improvements

- [#264](https://github.com/babylonlabs-io/babylon/pull/264) bump docker workflow
version to 0.10.2, fix some Dockerfile issues
- [#284](https://github.com/babylonlabs-io/babylon/pull/284) Update cosmos sdk math dependency
- [#285](https://github.com/babylonlabs-io/babylon/pull/285) Update cometbft dependency
version

### Bug fixes

- [#270](https://github.com/babylonlabs-io/babylon/pull/270) Validate there is only
one finality provider key in the staking request
- [#270](https://github.com/babylonlabs-io/babylon/pull/277) Panic due to possible
nil params response

## v0.16.1

### API Breaking

- [#273](https://github.com/babylonlabs-io/babylon/pull/273) Add full staking tx to BTC delegation creation event

## v0.16.0

### Improvements

* [#242](https://github.com/babylonlabs-io/babylon/pull/242) Add
ResumeFinalityProposal and handler
* [#258](https://github.com/babylonlabs-io/babylon/pull/258) fix go releaser
and trigger by github action
* [#252](https://github.com/babylonlabs-io/babylon/pull/252) Fix
flunky TestValidateParsedMessageAgainstTheParams
* [#229](https://github.com/babylonlabs-io/babylon/pull/229) Remove babylond
e2e before upgrade
* [#250](https://github.com/babylonlabs-io/babylon/pull/250) Add tests for
unbonding value mismatch
* [#249](https://github.com/babylonlabs-io/babylon/pull/249) Add logs to checking
unbonding output
* [#253](https://github.com/babylonlabs-io/babylon/pull/253) Upgrade cometbft dependency
* [#256](https://github.com/babylonlabs-io/babylon/pull/256) Removes retry library
from core Babylon repository
* [#257](https://github.com/babylonlabs-io/babylon/pull/257) Fix error handling
in checkpointing module
* [#262](https://github.com/babylonlabs-io/babylon/pull/262) Upgrade wasmd, relayer dependencies

### State Machine Breaking

* [#260](https://github.com/babylonlabs-io/babylon/pull/260) Finish tokenomics
implementation
* [#254](https://github.com/babylonlabs-io/babylon/pull/254) Avoid constant
bech-32 decoding in power table
* [#265](https://github.com/babylonlabs-io/babylon/pull/265) Add allow list for staking txs

## v0.15.0

### State Machine Breaking

* [#211](https://github.com/babylonlabs-io/babylon/pull/211) Babylon inflation module
* [#217](https://github.com/babylonlabs-io/babylon/pull/217) Move voting power table to finality module

### Improvements

* [#235](https://github.com/babylonlabs-io/babylon/pull/235) Change default values
for iavl cache when using `testnet` command

### API Breaking

* [238](https://github.com/babylonlabs-io/babylon/pull/238) Add additional data
to delegation creation event

## v0.14.1

### API Breaking

* [#228](https://github.com/babylonlabs-io/babylon/pull/228) Add inclusion height to early unbonding event

### State Machine Breaking

* [#218](https://github.com/babylonlabs-io/babylon/pull/218) Fix removing voting
power during expiry

## v0.14.0

### State Machine Breaking

* [#224](https://github.com/babylonlabs-io/babylon/pull/224) Make injected checkpoint a standard tx
* [#207](https://github.com/babylonlabs-io/babylon/pull/207) Rename total voting power
to total bonded sat
* [#204](https://github.com/babylonlabs-io/babylon/pull/204) Add babylon finality
activation block height to start processing finality messages in `x/finality` params.
* [#215](https://github.com/babylonlabs-io/babylon/pull/215) Implement ADR-29
generalized unbonding handler

### Improvements

* [#213](https://github.com/babylonlabs-io/babylon/pull/213) Bump wasmd and re-enable static linking
* [#210](https://github.com/babylonlabs-io/babylon/pull/210) Parameterize finality parameters in prepare-genesis cmd

## v0.13.0

### API Breaking

* [#194](https://github.com/babylonlabs-io/babylon/pull/194) Adjusted handling of `FinalityProviderSigningInfo` in finality keeper queries to improve API security.
  * Modified `QuerySigningInfosResponse` to remove direct exposure of sensitive fields.
  * Updated related tests in `x/finality/keeper/grpc_query_test.go`.
* [#200](https://github.com/babylonlabs-io/babylon/pull/200) Adjusted handling of `Gauge` in incentive keeper queries to improve API security.
* [#201](https://github.com/babylonlabs-io/babylon/pull/201) Adjusted handling of `ValidatorWithBlsKey` in checkpoint keeper queries to improve API security.
* [#202](https://github.com/babylonlabs-io/babylon/pull/202) Adjusted handling of `FinalityProviderWithMeta` in btcstaking keeper queries to improve API security.
* [#203](https://github.com/babylonlabs-io/babylon/pull/203) Adjusted handling of `RewardGauge` in incentive keeper queries to improve API security.
* [#208](https://github.com/babylonlabs-io/babylon/pull/208) Adjusted handling of `Evidence` in finality keeper queries to improve API security.

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
