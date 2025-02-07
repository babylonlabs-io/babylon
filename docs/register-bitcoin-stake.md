# Bitcoin Stake Registration
## Table of contents
1. [Introduction](#1-introduction)
2. [Stake registration on the Babylon chain](#2-stake-registration-on-the-babylon-chain)
   1. [Post-Staking Registration Flow](#21-post-staking-registration-flow)
   2. [Pre-Staking Registration Flow](#22-pre-staking-registration-flow)
3. [Stake Submission](#3-stake-submission)
   1. [Overview of Data that needs to be Submitted](#31-overview-of-data-that-needs-to-be-submitted)
   2. [Babylon Chain BTC Staking Parameters](#32-babylon-chain-btc-staking-parameters)
   3. [Creating the Bitcoin transactions](#33-creating-the-bitcoin-transactions)
   4. [The `MsgCreateBTCDelegation` Babylon message](#34-the-msgcreatebtcdelegation-babylon-message)
4. [Stake Registration Flows](#4-stake-registration-flows)
   1. [Post-Staking Registration](#41-post-staking-registration)
   2. [Pre-Staking Registration](#42-pre-staking-registration)
   3. [Technical Resources for Babylon Broadcasting](#43-technical-resources-for-babylon-broadcasting)
5. [Managing your Stake](#5-managing-your-stake)
   1. [On-demand unbonding](#51-on-demand-unbonding)
   2. [Withdrawing Expired/Unbonded BTC Stake](#52-withdrawing-expiredunbonded-btc-stake)
   3. [Withdrawing Remaining Funds after Slashing](#53-withdrawing-remaining-funds-after-slashing)
6. [Bitcoin Staking Rewards](#6-bitcoin-staking-rewards)

## 1. Introduction
This document walks through the communication protocol
with the Babylon chain in order to register existing Bitcoin stakes
or new ones. The document is structured as follows:
- [Section 2](#2-stake-registration-on-the-babylon-chain) provides an overview
  of the methods for registering stakes on the Babylon chain, detailing the two
  primary flows.
- [Section 3](#3-stake-submission) describes the data required for submission,
  the submission process, and introduces the `MsgCreateBTCDelegation` message used
  to communicate staking transactions to the Babylon chain. It also explains how
  to fill in its various fields.
- [Section 4](#4-stake-registration-flows) details the two main
  flows for registering stakes on the Babylon chain and how to submit the
  necessary data to the Babylon chain.
- [Section 5](#5-monitoring-your-stake) explains how to monitor the status of
  your stake transaction and provides an overview of the unbonding and withdrawal
  processes.

**Ways to Stake**
1. **Front-End Dashboard**:
Use the Babylon front-end dashboard for a user-friendly interface to manage your
 staking activities.
2. **Staker CLI**:
Utilize the command line interface (CLI) for more direct control over staking
operations. This is suitable for users comfortable with command-line tools.
3. **Custom Build**:
  * TypeScript: Create and manage staking transactions using TypeScript.
    Refer to the [simple-staking repository](https://github.com/babylonlabs-io/simple-staking/blob/main/src/app/hooks/services/useTransactionService.ts#L672-L679) for examples and broadcast to the Babylon
  network.
  * Golang Library: Use the [Golang library](https://github.com/babylonlabs-io/babylon/blob/main/x/btcstaking/types/tx.pb.go)
    to construct and broadcast staking transactions. See the Babylon repository
    for implementation details.

## 2. Stake registration on the Babylon chain

Babylon requires Bitcoin stakes to be registered to recognize them
and grant them voting power.
This is achieved through the submission
of specific data associated with the staker and the staking operation
to the Babylon chain. Through this process,
the Babylon chain validates that the staking operation is a valid one
and that the Bitcoin is staked in a protocol compliant manner.

There are two main flows for registering Bitcoin stakes with Babylon, depending
on the staker's circumstances:
the *post-staking registration* flow and
the *pre-staking registration* flow.
Each approach is tailored to meet the needs of different stakers, whether
they intend to register their BTC staking transaction after
they have submitted it on Bitcoin (post-staking registration, e.g.,
for phase-1 stakers) or
are creating stakes which they want the Babylon chain to validate
first in order to receive acceptance guarantees (pre-staking registration, e.g.,
for newly created stakes post Babylon chain launch).

### 2.1. Post-Staking Registration Flow

This flow is for stakers who have their BTC Staking transaction
already included in the Bitcoin ledger and subsequently registered on the
Babylon chain. For example,
this includes those that have participated in Babylon's Phase-1.
In this flow, the staker submits their BTC stake details along with any
additional data required by the Babylon Bitcoin Staking protocol
to the Babylon chain.

### 2.2. Pre-Staking Registration Flow

This flow is for stakers that first wish to register their stake to
the Babylon chain for verification before submitting the BTC Staking
transaction locking the stake to the Bitcoin ledger.
The staker starts by submitting all relevant data associated with their Bitcoin
staking to the Babylon chain prior to submitting the BTC Staking transaction
to the Bitcoin ledger. Once Babylon verifies the staking submission,
the covenant committee provides the necessary signatures for on-demand unbonding,
the staker gains the assurance needed to proceed with submitting their
stake to Bitcoin. After the BTC staking transaction receives sufficient
confirmations, the staker can submit a proof of inclusion to the Babylon
chain to obtain voting power based on their confirmed stake. Alternatively,
the staker can rely on the off-chain
[vigilante watcher](https://github.com/babylonlabs-io/vigilante) program,
which will submit the proof of inclusion on their behalf

## 3. Stake Submission

### 3.1. Overview of Data that needs to be Submitted

When registering a BTC stake on Babylon, the staker must prepare the following data
to either be submitted to Babylon and/or Bitcoin:

* **BTC Staking Transaction**: The transaction to be submitted to Bitcoin.
* **Unbonding Transaction**: The unbonding transaction that will be used by
  the staker to on-demand unbond their funds if they wish to do so and at a
  time of their choosing. This transaction is submitted *unsigned* by the staker
  to the Babylon chain in order to be co-signed by the covenant committee.
* **Consent to Slashing**: The staker must submit two pre-signed slashing
  transactions: one for the BTC Staking transaction and another for the unbonding
  transaction. These transactions require signatures from the staker,
  a quorum of the covenant committee, and the finality provider.
  Upon submission, the staker is required to submit the pre-signed slashing
  transactions, which will be co-signed by the covenant committee.
  This process ensures that if the finality provider double-signs, leading to
  the exposure of their private key, the slashing transaction can be fully
  signed and propagated to the Bitcoin network.
* **Proof of Possession**: Verifies ownership of the Bitcoin key by the
  Babylon account that is used for the stake registration.
* **Associated Staking Data**: Includes necessary metadata for the stake.
* **Merkle Proof of Inclusion (optional)**: Provides proof of transaction
  inclusion in a Bitcoin block.

The associated data needs to be bundled into a Babylon chain transaction
and broadcasted to the Babylon network. The exact flow
depends on whether the staker intends to follow the pre-staking registration
or post-staking registration flow.

The following sections cover:
- Required parameters for BTC Staking transactions.
- Construction of the Babylon transaction and required staking data.
- Submission and monitoring of the registration transaction on the Babylon chain.

### 3.2. Babylon Chain BTC Staking Parameters

BTC Staking transactions need to abide by the parameters specified by the Babylon
chain for BTC Staking transaction validity. These parameters are Bitcoin height-specific,
meaning that staking transactions included in different Bitcoin block heights
might correspond to different Bitcoin staking validity rules.
The parameters and the Bitcoin block heights they apply to are defined as follows:
* Each parameters version specifies a Bitcoin block height from which it starts applying
  known as the `btc_activation_height`.
* In order to find out which parameters version a Bitcoin transaction should adhere to,
  you take all the parameters versions and sort them in an ascending manner through the
  `btc_activation_height`.
* The first parameters version that has `btc_activation_height > lookup_btc_height` is the
  staking parameters version for the `lookup_btc_height` Bitcoin height.

Below, we provide
a brief overview of the parameters employed by the Babylon chain
as part of the [x/btcstaking](../x/btcstaking) Cosmos SDK module.
We will focus on the values defined in each parameters version:

* `covenant_pks`:
  A list containing the BIP-340 public keys used by the covenant committee.
  Expressed as a list of 64-character hex strings. The pks are an x-coordinate only
  representation of a secp256k1 curve point as the y-coordinate is implied.
* `covenant_quorum`:
  The minimum number of signatures to achieve a quorum from the
  covenant committee in uint32 format.
* `min_staking_value_sat`:
  The minimum amount of satoshis (smallest unit of Bitcoin) required to be
  locked in the staking output (i.e., staked) in order for the
  BTC staking transaction to be valid. Represented in int64 format.
* `max_staking_value_sat`:
  The maximum amount of satoshis (smallest unit of Bitcoin) required to be
  locked in the staking output (i.e., staked) in order for the
  BTC staking transaction to be valid. Represented in int64 format.
* `min_staking_time_blocks`:
  The minimum staking time as the number of Bitcoin blocks. This number
  should be added in the locktime specified in the staking output script.
  Represented in uint32 format.
* `max_staking_time_blocks`:
  The maximum staking time as the number of Bitcoin blocks. This number
  should be added in the locktime specified in the staking output script.
  Represented in uint32 format.
* `slashing_pk_script`:
  The `pk_script` expected in the slashing output, i.e., the first output of
  the slashing transaction. It is stored as a sequence of bytes, representing
  the conditions for spending the output.
* `min_slashing_tx_fee_sat`:
  The minimum transaction fee (in satoshis) required for the pre-signed slashing
  transaction. Represented in int64 format.
* `slashing_rate`: A scalar that defines what's the portion of the stake
  that will be slashed in the case of the finality provider double-signing.
* `unbonding_time_blocks`:
  The required time lock on unbonding transactions (and change output of
  slashing transactions).
* `unbonding_fee_sat`:
  The fee required for unbonding transactions.
* `min_commission_rate`: A scalar defining the minimum commission rate
  for finality providers.
* `delegation_creation_base_gas_fee`: A uint64 defining the minimum
  gas fee to be paid when registering a stake through the pre-staking
  registration flow.
* `allow_list_expiration_height`: The Babylon block height (as a uint64)
  on which the initial staking transaction allow-list expires.
* `btc_activation_height`: The Bitcoin block height on which this version of
  parameters takes into effect.

**How to Retrieve Parameters**

The above parameters are specified as the parameters of
the [x/btcstaking](../x/btcstaking) and can be retrieved by a connection
to a Babylon node (either through an RPC or LCD node connection).

### 3.3. Creating the Bitcoin transactions

The Bitcoin staking parameters defined in the previous section
can be used to create the Bitcoin transactions associated
with the BTC Staking protocol, as defined in the introductory section.
These transactions are all required for Bitcoin stakes to be registered
on Babylon:
* **BTC Staking Transaction**: This is a Bitcoin transaction that commits
  a certain amount of to-be-staked Bitcoin to Babylon-recognized
  BTC staking scripts. These scripts lock the stake for a chosen
  amount of BTC blocks and enable other features such as unbonding and
  slashable safety.
* **Slashing Transaction**: pre-signed transaction to consent to slashing in the case of
  double-signing.
* **Unbonding Transaction**: The unbonding transaction is used to
 on-demand unlock the stake before its originally committed timelock has expired.
* **Unbonding Slashing Transaction**: pre-signed transaction to consent to slashing
  in the case of double-signing during the unbonding process.

The following methods can be used to create the above Bitcoin transactions:
* [Golang BTC staking library](../btcstaking)
* [TypeScript BTC staking library](https://github.com/babylonlabs-io/btc-staking-ts)
* DIY implementation following the
  [staking script specification](./staking-script.md).

> **Note**: Please make sure to use the valid Babylon parameters when creating the above
> transactions with the libraries.

### 3.4. The `MsgCreateBTCDelegation` Babylon message

We use transactions in the Cosmos SDK, which provides
a framework for handling such messages. For more details, see the
[Cosmos SDK documentation](https://docs.cosmos.network/main/build/building-modules/messages-and-queries).

This specific message registers a BTC delegation with the Babylon chain along
with all the necessary data required to create/register a stake.

Below are the key fields and expectations from the
[`MsgCreateBTCDelegation` message](../proto/babylon/btcstaking/v1/tx.proto).

```proto
// MsgCreateBTCDelegation is the message for creating a BTC delegation
message MsgCreateBTCDelegation {
  option (cosmos.msg.v1.signer) = "staker_addr";
  // staker_addr is the address to receive rewards from BTC delegation.
  string staker_addr = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  // pop is the proof of possession of btc_pk by the staker_addr.
  ProofOfPossessionBTC pop = 2;
  // btc_pk is the Bitcoin secp256k1 PK of the BTC delegator
  bytes btc_pk = 3 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // fp_btc_pk_list is the list of Bitcoin secp256k1 PKs of the finality providers, if there is more than one
  // finality provider pk it means that delegation is re-staked
  repeated bytes fp_btc_pk_list = 4 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // staking_time is the time lock used in staking transaction
  uint32 staking_time = 5;
  // staking_value  is the amount of satoshis locked in staking output
  int64 staking_value = 6;
  // staking_tx is a bitcoin staking transaction i.e transaction that locks funds
  bytes staking_tx = 7 ;
  // staking_tx_inclusion_proof is the inclusion proof of the staking tx in BTC chain
  InclusionProof staking_tx_inclusion_proof = 8;
  // slashing_tx is the slashing tx
  // Note that the tx itself does not contain signatures, which are off-chain.
  bytes slashing_tx = 9 [ (gogoproto.customtype) = "BTCSlashingTx" ];
  // delegator_slashing_sig is the signature on the slashing tx by the delegator (i.e., SK corresponding to btc_pk).
  // It will be a part of the witness for the staking tx output.
  // The staking tx output further needs signatures from covenant and finality provider in
  // order to be spendable.
  bytes delegator_slashing_sig = 10 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
  // unbonding_time is the time lock used when funds are being unbonded. It is be used in:
  // - unbonding transaction, time lock spending path
  // - staking slashing transaction, change output
  // - unbonding slashing transaction, change output
  // It must be smaller than math.MaxUInt16 and larger that max(MinUnbondingTime, CheckpointFinalizationTimeout)
  uint32 unbonding_time = 11;
  // fields related to unbonding transaction
  // unbonding_tx is a bitcoin unbonding transaction i.e transaction that spends
  // staking output and sends it to the unbonding output
  bytes unbonding_tx = 12;
  // unbonding_value is amount of satoshis locked in unbonding output.
  // NOTE: staking_value and unbonding_value could be different because of the difference between the fee for staking tx and that for unbonding
  int64 unbonding_value = 13;
  // unbonding_slashing_tx is the slashing tx which slash unbonding contract
  // Note that the tx itself does not contain signatures, which are off-chain.
  bytes unbonding_slashing_tx = 14 [ (gogoproto.customtype) = "BTCSlashingTx" ];
  // delegator_unbonding_slashing_sig is the signature on the slashing tx by the delegator (i.e., SK corresponding to btc_pk).
  bytes delegator_unbonding_slashing_sig = 15 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
}
```
* `staker_addr`:
  This field is a Bech32-encoded Cosmos address, representing the
  staker's account where rewards will be accumulated. On the Babylon chain,
  the `bbn...` prefix is used
  (for example `bbn1x3pgec70smnl7vsdzz88asnv47fdntyn5u98yg`).
  This address must be used to sign the Babylon transaction
  and will be the destination of any Bitcoin staking rewards.
* `pop` (Proof of Possession):
  A cryptographic signature attesting to the ownership of the Bitcoin private key
  used for staking. This is required to confirm the legitimate
  ownership of the Bitcoin that is staked. Below is the full
  protobuf definition for the Proof of Possession, with the following
  fiels:
  * `btc_sig_type`: Defines what signature algorithm is used to produce the
    signature. It is an enum, with the available values
    being `0` (BIP-340), `1` (BIP-322), and `2` (ECDSA).
    More details on the algorithms:
    * [BIP-340 (Schnorr) spec](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
    * [BIP-322 spec](https://github.com/bitcoin/bips/blob/master/bip-0322.mediawiki)
      * Note that the `simple` signature format is used.
    * [ECDSA spec](https://github.com/bitcoin/bips/blob/master/bip-0137.mediawiki)
  * `btc_sig`: The signature of the `SHA-256` hash of the Babylon staker
    address (i.e., sign(sha256sum(`bbn...`)) using the previously specified algorithm
    in bytes. For example, you can use the following test vector,
    * **Babylon Staker Address**: `bbn1x3pgec70smnl7vsdzz88asnv47fdntyn5u98yg`
    * **SHA-256 hash of the address**: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (example hash)
    * **Signature using ECDSA (secp256k1)**:
      * Signature: `304402207a01b7f4c8f5d1f8053a5d1a74c2f14b5b8ef4b4c98b06d0eb82f7a5bb32f3dd02202d1e6e8f9f5a1b8e50e3829bcb3a2e85c6fa9c0c8c9cf1f5a3e2e3f8c3b2e4d5`
      * Public Key: `02a1633caf7b0e7e5ad5bcb6ff48a7c5d27eaa14a24e2a1b7bc8d5bdf58c6c2c9e`

```proto
message ProofOfPossessionBTC {
    // btc_sig_type indicates the type of btc_sig in the pop
    BTCSigType btc_sig_type = 1;
    // btc_sig is the signature generated via sign(sk_btc, babylon_staker_address)
    // the signature follows encoding in either BIP-340 spec or BIP-322 spec
    bytes btc_sig = 2;
}
```
> **Note**: While the `pop` field specifies the Bitcoin part of the signature,
> the Babylon part is implied. The staker will sign the transaction with their
> Babylon address, which is where the rewards will be directed. This ensures that
> the rewards are correctly associated with the staker's Babylon account.

* `btc_pk`:
  This field represents the Bitcoin `secp256k1` public key of the BTC staker
  in the BIP-340 format used for Schnorr signatures. It is a compact, 32-byte
  value derived from the staker's private key. This public key corresponds to
  the staker public used to construct the [staking script](./staking-script.md) used
  in the BTC Staking transaction.
* `fp_btc_pk_list`:
  A list containing the `secp256k1` public keys of the finality providers
  (FPs) to which the delegation is delegated. For phase-2,
  this list should only contain a single element, as in that phase
  Babylon is the only system secured by the Bitcoin stake.
  The public keys are in the BIP-340 format used for Schnorr
  signatures, that is a 32-byte value corresponding to the
  finality provider to which the stake is delegated to.
  These public key(s) should be exactly the same as the ones
  used when constructing the [staking script](./staking-script.md).
* `staking_time`:
  The duration of staking specified as a number of Bitcoin blocks.
  This duration should be the same as the one used by the staker
  as the timelock when creating their BTC Staking transaction.
  The value **must** comply with Babylon's parameters for the Bitcoin
  height in which the BTC Staking transaction is included,
  such as the minimum and maximum allowed staking time durations.
  Additionally, the type of this value must be an unsigned
  integer (uint32).
* `staking_value`:
  This field specifies the amount of satoshis locked in the staking
  output of the BTC Staking transaction.
  It is formatted as a signed integer (int64) and should be
  precisely the same as the staked amount in the BTC Staking transaction
  provided.
* `staking_tx`:
  The BTC Staking transaction is provided in hex format and can be either signed or
  unsigned. It is crucial that the transaction is constructed using exactly
  both the Babylon network parameters and the data provided
  above. More specifically:
  * The covenant keys & quorum should correspond to the ones defined
    in the Babylon network parameters.
  * Staker public key used should be the same as the `btc_pk` specified above.
  * The finality provider list should be the same as the `fp_btc_pk_list`
    specified above.
  * The staking timelock should be the same as the `staking_time` above.
  * The staking amount in satoshis should be the same as the `staking_value` above.
* `staking_tx_inclusion_proof` (Optional):
  The field corresponds to the staking transaction inclusion proof on the Bitcoin ledger.
  It should be set when going through the
  [Bitcoin-first flow](./register-bitcoin-stake#21-bitcoin-first-flow).
  It is a Merkle proof showing that the BTC Staking transaction is included in the
  Bitcoin chain. It is defined as an `InclusionProof` protobuf data type (specified below)
  with the following fields:
  * `key`: Identifies the transaction's position in the Bitcoin blockchain.
    The key should correspond to the `TransactionKey` type (defined below),
    which contains two fields:
    * `txIdx`: The index of the transaction within the block (e.g. tx number 42).
    * `blockHash`: The hash of the block containing the transaction.
  * `proof`: A Merkle proof verifying the transaction's inclusion in the Bitcoin chain.
    It is a list of transaction hashes the staking transaction hash is paired with,
    recursively, in order to trace up to obtain a merkle root of the block, deepest
    pairing first. You can refer to
    [this document](https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get-merkle)
    for more details. This proof is crucial for
    demonstrating that the transaction has been confirmed, allowing the Babylon
    chain to recognize and register the stake.
    To generate the proof you can use the [Golang BTC staking library](../btcstaking).
  ```proto
  // in x/btcstaking module types
  message InclusionProof {
      // key is the position (txIdx, blockHash) of this tx on BTC blockchain
      babylon.btccheckpoint.v1.TransactionKey key = 1;
      // proof is the Merkle proof that this tx is included in the position in `key`
      bytes proof = 2;
  }

  // in x/btccheckpoint module types
  message TransactionKey {
    uint32 index = 1;
    bytes hash = 2
      [ (gogoproto.customtype) =
      "github.com/babylonlabs-io/babylon/types.BTCHeaderHashBytes" ];
  }
  ```
* `slashing_tx`:
  The slashing transaction that spends the BTC Staking transaction in hex format.
  It can be constructed based on the details of the previous section [here](#33-creating-the-bitcoin-transactions).
  Note that this slashing transaction is different than the one
  used to spend the unbonding transaction.
  A valid pre-signature for this transaction must be included in the
  `delegator_slashing_sig` field.
* `delegator_slashing_sig`:
  The signature for the `slashing_tx` in hex format, which is crucial for authorizing the
  transaction in the event of a protocol violation, such as double-signing.
  The signature, generated with the private key for the staker's secp256k1
  public key (`btc_pk`), should be compatible with Schnorr (BIP340).
* `unbonding_time`:
  The timelock period for unbonding, measured in Bitcoin blocks, indicating
  how long funds remain locked. It should be an unsigned integer (uint32) and
  ensure that any `unbonding_time` you choose is more than that specified
  by the `min_unbonding_time_blocks` parameters in the `btcstaking` module.
* `unbonding_tx`:
  The unsigned unbonding transaction in hex format. The submission of the unbonding
  transaction is a requirement in order to (1) receive the covenant signatures for
  the unbonding transaction and (2) for the verification of the slashing
  transaction that spends the unbonding one.
* `unbonding_value`:
  The amount of satoshis committed to the unbonding output of the unbonding transaction.
* `unbonding_slashing_tx`:
  The slashing transaction that spends the unbonding transaction in hex format.
  It can be constructed based on the details of the previous section [here](#33-creating-the-bitcoin-transactions).
  Note that this slashing transaction is different compared to the one
  used to spend the BTC Staking transaction.
  A valid pre-signature for this transaction must be included in the
  `delegator_unbonding_slashing_sig` field.
* `delegator_unbonding_slashing_sig`:
  The unbonding slashing signature for the delegator in hex format. This must be a valid
  signature generated using the private key corresponding to the public key of the
  Bitcoin staker (`btc_pk`).

### 3.5. Constructing the `MsgCreateBTCDelegation`

There are several methods to construct and communicate the `MsgCreateBTCDelegation`
message to the Babylon network:
* Command line interface (CLI), through the `babylond tx btcstaking create-btc-delegation`
  command.
* Creation using TypeScript based on this [reference implementation](
  https://github.com/babylonlabs-io/simple-staking/blob/2b9682c4f779ab39562951930bc3d023e5467461/src/app/hooks/services/useTransactionService.ts#L672-L679)
  and broadcast to the Babylon network.
* Creation using Golang based on this [type reference](../x/btcstaking/types/tx.pb.go)
  and broadcast to the Babylon network.

> **Important**: When submitting a staking transaction using the
> pre-registration staking flow a special gas fee should be applied.
> More details on this in the following section.

## 4. Stake Registration Flows

Having defined the construction of Bitcoin staking transactions and their
encapsulation into the `MsgCreateBTCDelegation` message type, we now turn our
attention to utilizing these data structures to fully register stakes on the
Babylon chain, following the various flows outlined in Section 2.

### 4.1. Post-Staking Registration

This flow relates to stakes that have already been included in a confirmed
Bitcoin block and intend to register on the Babylon chain
(such as phase-1 stakes).

The following diagram explains the post-staking registration flow
![stake-registration-post-staking-flow](./static/postregistration.png)

1. Create the necessary metadata for the creation of `MsgCReationBTCDelegation`
   such as the proof of inclusion, the staking, unbonding, and slashing transactions,
   as well as the proof of possession.
2. Construct the `MsgCreateBTCDelegation` with the proof of inclusion filled set.
> **Note**: The proof of inclusion being filled is essential
>  to prove the that the Bitcoin stake has been included in a deep
>  enough block (k-deep, see the [btccheckpoint](../x/btccheckpoint)
>  module spec).
3. Wait for the covenants to add their verification signatures. Until they do so,
   the stake will have the status of `PENDING`.
4. Once the stake receives
   a quorum of signatures it will be designated as `ACTIVE`.

### 4.2. Pre-Staking Registration

This flow is used by stakers that want to receive verification of their
stake transactions' validity as well as receive the on-demand unbonding
transaction covenant signatures before locking their stake on the Bitcoin ledger.
It is most suitable for stakers generating new stakes after the Babylon chain
has been launched.

![stake-registration-pre-staking-flow](./static/preregistration.png)

1. Construct the `MsgCreateBTCDelegation` without the Proof of Possession (PoP)
   field set.
> **Note**: Omitting the optional proof of possession field indicates
> that the stake is initially submitted for verification without being
> submitted to Bitcoin. For the stake to become active,
> the proof of inclusion on a k-deep Bitcoin block will need to be submitted
> at a later point.
> Typically, the [vigilante watcher](https://github.com/babylonlabs-io/vigilante)
> program will perform this service.
> but the staker can also submit the proof of inclusion themselves.
2. Wait for the covenants to add their verification signatures. Until
   then the stake will be labeled as `PENDING`.
3. Once the stake receives a quorum of signatures it will be labeled as `VERIFIED`.
   This denotes that a quorum of covenant signatures has been submitted for the
   slashing and on-demand unbonding Bitcoin transactions associated with the BTC stake.
4. Following verification, the staker is confident to sign
   the BTC Staking transaction and broadcast it to the Bitcoin network.
5. The [Vigilante Watcher](https://github.com/babylonlabs-io/vigilante) service
   will receive identify that a staking transaction is waiting Bitcoin confirmation
   and will start monitoring Bitcoin for its inclusion and subsequent k-deep inclusion.
6. The vigilante watcher will identify that the transaction is k-deep on the Bitcoin chain.
7. Subsequently, the vigilante watcher will construct
   will construct a `MsgAddBTCDelegationInclusionProof` that includes a proof of inclusion
   of the staking transaction in a k-deep block (you can find more details about this
   message [here](../x/btcstaking)).
> **Note**: If you do not trust the vigilante wathcer service to timely deliver
> a notification to the Babylon chain about your stake's inclusion in a k-deep block,
> you can monitor for such inclusion yourself and submit the `MsgAddBTCDelegationInclusionProof`
> message on your own.
> **k-depth**: `k` is a protocol level parameter
> specifying Bitcoin block inclusion depth, which is defined as
> the difference between the tip height and the height of the Bitcoin block in question
> (e.g. if the Bitcoin tip is height 100, block 99 is 1-deep).
> For Babylon testnets, this is typically set to 10, and can be retrieved
> by the [btccheckpoint](../x/btccheckpoint) module's parameters.
8. After receiving the proof of inclusion, the stake is designated as `ACTIVE`. 

**Important: Gas requirements for the pre-staking registration flow**:
Given that the pre-staking registration flow does not have the requirement
that the staker has already committed funds to the Bitcoin network,
it could serve as a chain spamming vector, especially given that it
leads to the submission of multiple covenant emulator signatures.
To combat this, the submission of a `MsgCreateBTCDelegation` message
using the pre-staking registration flow requires a minimum gas amount
specified by the `delegation_creation_base_gas_fee` attribute of the
Babylon parameters.

**Important: Which Bitcoin Staking parameters to use**: Given that the staking
parameters are Bitcoin block height specific and the fact that the
pre-staking registration flow requires the staker to first submit their
transaction to Babylon and then to Bitcoin, a concern might arise that
the Bitcoin block height at the time of Babylon submission might correspond
to different parameters than the Bitcoin block height at the time of
Bitcoin inclusion. To combat this, the Babylon chain expects
that the staking transaction will use the Bitcoin staking parameters
defined for the Bitcoin height of the tip of the **on-chain Bitcoin light client**
of the Babylon chain at the time of the pre-staking registration submission.
The tip height of the on-chain Bitcoin light client can be retrieved as follows:
* LCD/RPC: through the `/babylon/btclightclient/v1/tip` query endpoint
* CLI: through the `babylond query btclightclient tip` query

### 4.3. Technical Resources for Babylon Broadcasting

To broadcast your Babylon transactions, you will need access to a node and can use
various methods:

* Node Access: Ensure you have access to a Babylon node to broadcast your
transactions.
* Command Line Interface (CLI): Use the CLI for direct transaction submission.
* gRPC: Utilize gRPC for programmatic access to the network.
* External References: For detailed instructions, refer to external
[Cosmos SDK resources](https://docs.cosmos.network/main/learn/advanced/transactions#broadcasting-the-transaction), which provide comprehensive
guidance on using these tools.

## 5. Managing your Stake

### 5.1. On-demand unbonding

On-demand unbonding enables stakers to initiate the unbonding of their staked
BTC before the original timelock they have committed to in their staking transaction
expires. The funds become available after an unbonding period that is specified
in the Babylon parameters (see Section 3.2.).

Users can on-demand unbond by utilizing the same on-demand unbonding transaction
they submitted as part of their transaction registration, but
only after they fill in the required signature set which involves:
* The staker's signature, and
* signatures from a quorum of the covenant committee

The covenant committee signatures are filled in and committed on-chain as part
of the activation process of the stake. The stake will never be activated
(or verified in case of the pre-staking registration flow) unless it receives
a quorum of covenant signatures for the unbonding.

Following, we define the steps for retrieving the unbonding transaction and the covenant
signatures as well as adding your own and combining all to have a completely signed
unbonding transaction:

1. Query the delegation from the Babylon chain in order to gather the unbonding
transaction and the covenant signatures. For example, using the CLI:
```shell
babylond query btcstaking delegation [staking_tx_hash_hex]
```
2. Add your signature to the unbonding transaction witness.
3. Add the covenant signatures to the unbonding transaction witness.
4. The transaction is fully signed! You can now broadcast it to the Bitcoin network.

You can find a practical example on how the unsigned unbonding transaction and signatures
are combined to generate the fully signed unbonding transaction:
* [in our TypeScript library documentation](https://github.com/babylonlabs-io/btc-staking-ts?tab=readme-ov-file#create-unbonding-transaction), or
* in our [Golang staking library utils](../btcstaking/witness_utils.go).

**Unbonding notification on the Babylon chain**: The Babylon system employs
the [vigilante unbonding watcher](https://github.com/babylonlabs-io/vigilante) service
which is responsible for monitoring the Bitcoin ledger for on-demand unbonding transaction
inclusion and reporting it back to Babylon. This means, that as soon as your unbonding
transaction is included on Bitcoin, the vigilante service will pick it up and notify
the Babylon chain, leading to your stake losing its voting power.

### 5.2. Withdrawing Expired/Unbonded BTC Stake

The withdrawal process involves the submission of a Bitcoin transaction
that extracts Bitcoin stake for which the staking/unbonding timelock
has expired from the Bitcoin Staking/Unbonding script.
The process involves the retrieval of the corresponding transaction
with the expired timelock (either staking or unbonding), the
construction of a withdrawal transaction signed by the staker, and
finally the submission of the transaction to the Bitcoin blockchain.

You can find a practical example on how the withdrawal transaction is constructed:
* [in our TypeScript library documentation](https://github.com/babylonlabs-io/btc-staking-ts?tab=readme-ov-file#withdrawing), or
* in our [Golang staking library utils](../btcstaking/witness_utils.go).

### 5.3. Withdrawing Remaining Funds after Slashing

Bitcoin stake will be slashed if the finality provider to which
it has been delegated to double signs. Slashing involves the broadcast
of a [Bitcoin slashing transaction](./staking-script.md) that sends
a portion of the slashed funds to a burning address (portion defined in the
Bitcoin staking params defined in Section 3.2.), while the remaining funds
are put into a timelock script and can be unlocked using exactly the same
withdrawal transaction as described in the previous section.

## 6. Bitcoin Staking Rewards

As a reward for the economic security the Bitcoin staker provides,
they are rewarded with a native tokens of the chain
they secure. The rewards are distributed as follows:
* Upon each new block, a certain number of native tokens are minted as rewards.
* The rewards are split into the following three stakeholders:
  * Native stakers
  * Bitcoin stakers
  * Community pool
* For Bitcoin stakers, the rewards are further distributed based on the voting power
  and the commission of the finality provider they have delegated to.
* Bitcoin staking rewards are entered into a gauge and can be queried by the staker
  and withdrawn through the submission of a transaction.

**Querying for available rewards**: Rewards can be queried through the `x/incentive` module:
* Through RPC/LCD queries on the URL `/babylon/incentive/address/{address}/reward_gauge`,
  where `address` is the bech32 address of the staker.
* Through the CLI with the command `babylond query incentive reward-gauges <bech32-address>`

**Withdrawing rewards**: Rewards can be retrieved through the submission of the `MsgWithdrawReward` messges:
```protobuf
// MsgWithdrawReward defines a message for withdrawing reward of a stakeholder.
message MsgWithdrawReward {
    option (cosmos.msg.v1.signer) = "address";
    // {finality_provider, btc_delegation}
    string type = 1;
    // address is the address of the stakeholder in bech32 string
    // signer of this msg has to be this address
    string address = 2;
}
```

The messsage defines the following fields:
* `type`: Defines the stakeholder for which the rewards are withdrawn.
  This can be either `finality_provider` or `btc_delegation`.
* `address`: Defines the address for which the rewards are withdrawn.
  This address should be the same as the signer of the message.

One can withdraw rewards by either:
* Submitting the `MsgWithdrawReward` on any RPC/LCD node
* Using the CLI `babylond tx incentive withdraw-reward <type>`