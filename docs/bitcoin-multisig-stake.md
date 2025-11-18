# Bitcoin Multisig Stake

## Table of Contents

1. [Introduction](#1-introduction)
   1. [Why Multisig](#11-why-multisig)
   2. [Terminology](#12-terminology)
2. [Architecture Overview](#2-architecture-overview)
   1. [Taproot Script Layout](#21-taproot-script-layout)
   2. [On-chain Metadata](#22-on-chain-metadata)
3. [Parameters and Limits](#3-parameters-and-limits)
4. [Preparing a Multisig Delegation](#4-preparing-a-multisig-delegation)
   1. [Collect Delegator Material](#41-collect-delegator-material)
   2. [Construct `AdditionalStakerInfo`](#42-construct-additionalstakerinfo)
   3. [Sample JSON Payload](#43-sample-json-payload)
5. [Registering a Multisig BTC Delegation](#5-registering-a-multisig-btc-delegation)
   1. [End-to-End Flow](#51-end-to-end-flow)
   2. [Babylon Messages](#52-babylon-messages)
6. [Managing Multisig Delegations](#6-managing-multisig-delegations)
   1. [Stake Extension](#61-stake-extension)
   2. [On-demand Unbonding and Slashing](#62-on-demand-unbonding-and-slashing)
7. [Observability and Troubleshooting](#7-observability-and-troubleshooting)
   1. [Queries and Events](#71-queries-and-events)
   2. [Common Pitfalls](#72-common-pitfalls)

---

This document explains how multisignature (multisig) Bitcoin delegations are
constructed, registered, and operated on Babylon. It complements the
[Bitcoin stake registration](./register-bitcoin-stake.md) and
[stake extension](./bitcoin-stake-extension.md) guides by focusing on the data
and validation rules that are unique to M-of-N BTC stakers.

## 1. Introduction

### 1.1. Why Multisig

Many institutional or custodial stakers require shared control over their
staking keys. Babylon supports Taproot-based M-of-N multisig delegations so
that organizations can:

- split signing authority across multiple HSMs or operators
- enforce recovery policies without sacrificing liveness guarantees
- keep the same security story as self-custodial single-signature stakers

Multisig delegations reuse the same staking, unbonding, and slashing flows as
single-sig stakers. The difference lies in how the Bitcoin scripts and Babylon
messages commit to multiple staker keys and signatures.

### 1.2. Terminology

- **Primary staker**: the `btc_pk` field in Babylon messages. This key produces
  the proof-of-possession (PoP) and anchors delegation ownership on-chain.
- **Additional stakers**: the remaining `N-1` keys that participate in the
  multisig quorum. These keys are carried in `AdditionalStakerInfo`.
- **Staker quorum (`M`)**: the number of staker signatures required to satisfy
  the Bitcoin script (e.g., 2 for a 2-of-3 multisig).
- **Staker count (`N`)**: the total number of staker keys participating in the
  multisig (primary key + `staker_btc_pk_list`).
- **AdditionalStakerInfo**: the protobuf object that carries additional keys
  and their slashing signatures. See
  [`proto/babylon/btcstaking/v1/btcstaking.proto`](../proto/babylon/btcstaking/v1/btcstaking.proto).

## 2. Architecture Overview

### 2.1. Taproot Script Layout

Multisig delegations use the same Taproot tree as single-signature stakers:

```
                Taproot output
                    |
        ┌───────────┴────────────┐
        │                        │
  Time-lock path         Cooperative paths
                                  |
                     ┌────────────┴────────────┐
                     │                         │
              Unbonding path           Slashing path
```

The difference is the content of each script leaf:

- **Time-lock path**: replaces the single `OP_CHECKSIG` with an
  `OP_CHECKSIGADD` sequence that enforces the `M-of-N` relative timelock spend.
- **Unbonding path**: requires `M` delegator signatures plus the covenant quorum.
- **Slashing path**: requires `M` delegator signatures, one finality provider
  signature, and the covenant quorum.

The Bitcoin scripts are generated through
`btcstaking.BuildMultisigStakingInfo/BuildMultisigUnbondingInfo` which sort the
keys lexicographically and build a Taproot tree with three leaves. Because the
scripts use `OP_CHECKSIGADD`, the witness must contain one stack element per
staker key (empty entries stand for “no signature”). This detail is important
when constructing witnesses for spending staking or unbonding outputs.

### 2.2. On-chain Metadata

The Babylon chain stores the multisig configuration inside
`BTCDelegation.multisig_info`. The data includes:

- `staker_btc_pk_list`: the `N-1` additional keys (primary key excluded)
- `staker_quorum`: the quorum `M`
- `delegator_slashing_sigs`: signatures for the staking slashing transaction
- `delegator_unbonding_slashing_sigs`: signatures for the unbonding slashing
  transaction

This metadata allows the chain to verify:

- the Bitcoin scripts indeed commit to the declared key set
- Babylon holds at least `M` valid signatures for both slashing transactions
- no duplicate keys exist across staker, covenant, and finality provider sets

Once stored, the metadata is exposed through the
`babylon.btcstaking.v1.Query/BTCDelegation` response so wallet software and
monitoring systems can fetch the current quorum, keys, and signatures.

## 3. Parameters and Limits

Multisig delegations are bounded by two module parameters defined in
[`proto/babylon/btcstaking/v1/params.proto`](../proto/babylon/btcstaking/v1/params.proto):

- `max_staker_num`: maximum allowed `N`
- `max_staker_quorum`: maximum allowed `M`

Additional constraints enforced by the keeper (`x/btcstaking/types/validate_parsed_message.go`):

- `staker_quorum ≥ 1` and `N ≥ 2` – multisig mode requires at least two keys
- `staker_quorum ≤ N`
- the combination `M-of-N` must be within the configured parameter bounds
- the additional staker list **must not** contain the primary staker key and
  **must not** contain duplicates
- Babylon must receive ≥ `M` signatures for both staking and unbonding slashing
  transactions before accepting the delegation

These limits are governance-controlled and can be inspected via
`babylond q btcstaking params`.

## 4. Preparing a Multisig Delegation

### 4.1. Collect Delegator Material

Besides the standard data described in
[Bitcoin Stake Registration](./register-bitcoin-stake.md#3-bitcoin-stake-registration),
a multisig delegation requires the following preparatory steps:

1. **Generate keys**: produce the primary staker key (owner) plus the `N-1`
   additional staker keys. Each key must be a BIP-340 Schnorr key.
2. **Proof of possession**: only the primary key provides the PoP (`pop`
   field). Additional keys do not require a PoP.
3. **Sign slashing transactions**: every staker key must sign both the staking
   slashing transaction and the unbonding slashing transaction. These
   signatures are embedded in `AdditionalStakerInfo`.
4. **Agree on quorum**: decide the `M-of-N` threshold that will protect the
   funds on Bitcoin. The Babylon keeper verifies that this quorum matches the
   provided signatures and parameter limits.

### 4.2. Construct `AdditionalStakerInfo`

`AdditionalStakerInfo` mirrors the protobuf definition:

- `staker_btc_pk_list`: array containing every additional staker public key.
  The primary key **must not** be included here because it is already provided
  in `btc_pk`.
- `staker_quorum`: the requested `M`.
- `delegator_slashing_sigs`: list of `SignatureInfo { pk, sig }` records for
  the staking slashing transaction. Provide one element per additional staker
  key that signed. Babylon ensures at least `M` signatures exist overall
  (primary signature is collected separately).
- `delegator_unbonding_slashing_sigs`: same structure but for the unbonding
  slashing transaction.

All signatures must be encoded as BIP-340 Schnorr signatures. The `pk` fields
must match the associated signatures; otherwise `ErrInvalidMultisigInfo` is
raised when the message is parsed.

### 4.3. Sample JSON Payload

Below is an illustrative JSON fragment that can be embedded in a `MsgCreateBTCDelegation`
or `MsgBtcStakeExpand` payload:

```json
{
  "staker_btc_pk_list": [
    "228ab7c4...f58",  // staker #2
    "2fa1b042...9cd"   // staker #3
  ],
  "staker_quorum": 2,
  "delegator_slashing_sigs": [
    {
      "pk": "228ab7c4...f58",
      "sig": "a1b3...9ce"
    },
    {
      "pk": "2fa1b042...9cd",
      "sig": "bb44...1db"
    }
  ],
  "delegator_unbonding_slashing_sigs": [
    {
      "pk": "228ab7c4...f58",
      "sig": "c01d...55a"
    },
    {
      "pk": "2fa1b042...9cd",
      "sig": "de91...730"
    }
  ]
}
```

> **Note**: the primary staker key is implicit. Babylon will insert its
> signatures into the canonical key–signature maps before verifying the
> Bitcoin transactions.

## 5. Registering a Multisig BTC Delegation

### 5.1. End-to-End Flow

The high-level flow follows the same steps as single-sig delegations
(see [Section 2](./register-bitcoin-stake.md#2-bitcoin-stake-registration)):

1. Build staking/unbonding Bitcoin transactions that include multisig scripts.
2. Gather slashing signatures from all staker keys.
3. Submit a `MsgCreateBTCDelegation` containing the Bitcoin transactions,
   PoP, finality provider list, and the `multisig_info` payload.
4. Wait for the covenant quorum to attest (`PENDING → VERIFIED`).
5. Broadcast the staking transaction on Bitcoin (pre-staking flow) or provide
   an inclusion proof (post-staking flow).
6. Submit/relay `MsgAddBTCDelegationInclusionProof` once the transaction is
   `k`-deep so the delegation becomes `ACTIVE`.

The key differences are the additional multisig metadata, signature checks, and
Witness requirements enforced by the Babylon keeper.

### 5.2. Babylon Messages

- **`MsgCreateBTCDelegation`**: accepts an optional `multisig_info` field.
  When provided, the keeper validates quorum bounds, duplicates, and verifies
  that the supplied signatures match the slashing transactions. The rest of
  the message matches the single-sig flow.
- **`MsgBtcStakeExpand`**: also exposes a `multisig_info` field so that existing
  multisig delegations can extend their staking amount or timelock. The new
  staking transaction must spend the previous staking output (input index 0)
  and include the same finality provider set. Babylon rebuilds the multisig
  scripts using the provided keys to verify the covenant signatures before
  they are refunded.
- **`MsgAddCovenantSigs`**: unchanged, but covenant signatures are now applied
  over multisig scripts. Babylon automatically includes the multisig witness
  paths when verifying adaptor signatures.
- **`MsgBTCUndelegate`**: when a multisig delegation unbonds early, the message
  must include a witness where the staker signature vector contains one entry
  per multisig participant (empty byte arrays for non-signers). The keeper
  reconstructs the staker key set from `multisig_info` and validates the
  Schnorr signatures accordingly.

## 6. Managing Multisig Delegations

### 6.1. Stake Extension

Stake extension for multisig delegations follows the same rules described in
[Bitcoin Stake Extension](./bitcoin-stake-extension.md):

- The previous staking transaction must be `ACTIVE`.
- The new transaction must have exactly two inputs (old staking output and the
  funding UTXO) and at least one output recreating the staking Taproot script.
- The staking amount must be ≥ the previous amount.
- The finality provider list cannot change.

Additional multisig-specific considerations:

- The staker key set and quorum can change only if the new `multisig_info`
  satisfies Babylon parameters and provides `M` signatures for the new
  slashing transactions. Even if the staker set changes, Babylon will re-run
  the same validation path as for brand-new delegations.
- Covenant overlap requirements apply exactly as in the single-sig flow.

### 6.2. On-demand Unbonding and Slashing

Multisig stakers can unbond early by signing the registered unbonding
transaction (or by broadcasting a stake-spending transaction). The keeper
(`MsgBTCUndelegate`) enforces:

- the spending transaction actually consumes the staking output
- the witness includes signatures from `M` staker keys, reconstructed from
  `btc_pk` + `multisig_info`
- the funding transactions referenced in the witness are valid

During slashing events, Babylon uses the stored `delegator_slashing_sigs` /
`delegator_unbonding_slashing_sigs` vectors to assemble full witnesses together
with the covenant and finality provider signatures. Because OP_CHECKSIGADD
requires **exactly** one witness element per key, the keeper stores the
signatures in the order returned by `buildMultiSigScript`. Wallets must follow
the same ordering when broadcasting on Bitcoin (fill missing signatures with
empty byte slices).

## 7. Observability and Troubleshooting

### 7.1. Queries and Events

- `babylond q btcstaking btc-delegation <staking_tx_hash> --output json`
  returns `multisig_info` together with the delegation status, inclusion info,
  and stake expansion data.
- `babylond q btcstaking params` shows `max_staker_num` and
  `max_staker_quorum`, which help diagnose rejection errors.
- `EventBTCDelegationCreated` now emits a `multisig_staker_btc_pk_hexs`
  attribute listing the additional staker keys. This is useful for explorers
  and audit tooling. The event fires both for initial registrations and stake
  extensions.

### 7.2. Common Pitfalls

- **Primary key duplication**: ensure the `btc_pk` provided in the message does
  not appear inside `staker_btc_pk_list`. Babylon rejects duplicated keys with
  `ErrDuplicatedStakerKey`.
- **Insufficient slashing signatures**: at least `M` signatures are required
  for both slashing transactions. If a cosigner refuses to sign, the
  delegation cannot be registered.
- **Witness ordering**: when spending multisig staking or unbonding outputs on
  Bitcoin, the witness must contain exactly `N` entries (empty entries for
  non-signers). This matches the order returned by the Taproot script builder.
- **CLI support**: the current `babylond tx btcstaking create-btc-delegation`
  command does not provide convenience flags for `multisig_info`. Integrations
  should construct the message via gRPC/REST or extend the CLI to include the
  additional JSON payload.

For any other issues, consult the module-level documentation under
`x/btcstaking` or inspect the keeper logic around `buildMultisigStakingInfo`
and `MsgBTCUndelegate` to understand the exact validation error.
