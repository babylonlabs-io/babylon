# BTCStaking

The BTC staking module is responsible for maintaining the set of finality
providers and BTC delegations under them. This includes:

- handling requests for creating finality providers,
- handling requests for creating BTC delegations,
- handling requests for submitting signatures of covenant emulators,
- handling requests for unbonding BTC delegations, and
- proactively refreshing the active set of finality providers and BTC
  delegations.

## Table of contents

- [Table of contents](#table-of-contents)
- [Concepts](#concepts)
  - [Actors](#actors)
  - [Stake Creation](#stake-creation)
  - [Voting Power and Finality](#voting-power-and-finality)
  - [On-Demand Unbonding Tracking](#on-demand-unbonding-tracking)
- [States](#states)
  - [Parameters](#parameters)
  - [Finality providers](#finality-providers)
  - [BTC delegations](#btc-delegations)
  - [BTC delegation index](#btc-delegation-index)
- [Messages](#messages)
  - [MsgCreateFinalityProvider](#msgcreatefinalityprovider)
  - [MsgEditFinalityProvider](#msgeditfinalityprovider)
  - [MsgCreateBTCDelegation](#msgcreatebtcdelegation)
  - [MsgAddBTCDelegationInclusionProof](#msgaddbtcdelegationinclusionproof)
  - [MsgAddCovenantSigs](#msgaddcovenantsigs)
  - [MsgBTCUndelegate](#msgbtcundelegate)
  - [MsgUpdateParams](#msgupdateparams)
  - [MsgSelectiveSlashingEvidence](#msgselectiveslashingevidence)
- [BeginBlocker](#beginblocker)
- [Events](#events)
  - [Finality provider events](#finality-provider-events)
  - [Delegation events](#delegation-events)
- [Queries](#queries)

## Concepts

Babylon's Bitcoin Staking protocol allows bitcoin holders to _trustlessly_ stake
their bitcoin for providing economic security to the Babylon chain and other
Proof-of-Stake (PoS) blockchains, _without bridging their bitcoin elsewhere_.

### Actors
The following actors interact with the BTC staking module: 
- **BTC stakers (aka delegators)** delegate their bitcoin to a finality
  provider in order to obtain provide economic security to the PoS system.
  They interact with the `x/btcstaking` module to express their
  interest to stake, notify about their staking receiving relevant confirmations,
  or performing both of the previous steps (staking + staking confirmation notification)
  at once.
- **Finality providers** receive bitcoin delegations and participate in the
  _finality vote round_ on top of the CometBFT consensus.
  They interact with the `x/btcstaking` module in order
  to register or update their finality provider keys and information. For the
  remainder of their operations (voting, submitting public randomness),
  finality providers interact with the [`x/finality`](../x/finality) module.
- **Covenant emulators** who serve as
  [covenants](https://covenants.info) to enforce spending conditions on bitcoin
  staked on Babylon.
  [Covenant emulators](https://github.com/babylonlabs-io/covenant-emulator)
  interact with the `x/btcstaking` module to submit covenant signatures for the
  slashing, unbonding, and unbonding slashing transactions.
- **Vigilantes**: [Vigilante BTC Staking Trackers](https://github.com/babylonlabs-io/vigilante)
  monitor the Bitcoin staking ledger to identify whether staking transactions have been unbonded
  or whether a staking transaction the staker intended to create has been included and confirmed
  in the Bitcoin ledger.


### Stake Creation

#### Post-staking Registration
A Bitcoin staker can receive voting power through their Bitcoin stake delegation
by following this process:
1. Create a Bitcoin staking transaction and submit it to Bitcoin. The
   staking transaction locks the staker's bitcoin for a pre-determined
   timelock and specifies slashing conditions.
2. The BTC staker constructs the following transactions (whose specifications
   can be found [here](../../docs/staking-script.md)):
    - a pre-signed _slashing transaction_ that can spend the staking transaction once the
      finality provider is slashed,
    - an unsigned _unbonding transaction_ that spends the staking transaction to start
      the early unbonding process, and
    - a pre-signed _unbonding slashing transaction_ that can spend the unbonding
      transaction once the finality provider is slashed.
3. Once the staking transaction is confirmed on Bitcoin, the BTC staker sends
   the staking transaction, its inclusion proof, slashing transaction,
   unbonding transaction, and unbonding slashing transaction to Babylon.
   This happens by the submission of a [MsgCreateBTCDelegation](#msgcreatebtcdelegation) message.
4. The covenant emulators verify the transactions and submit their pre-signatures
   for the slashing transactions and unbonding transaction.
   The BTC Delegation is now activated.

#### Pre-staking Registration

The above mechanism requires the staker to first lock their funds
and then request the Babylon blockchain to activate the stake.
For stakers that want to avoid this and prefer to first receive confirmation
and then lock their funds on Bitcoin, the pre-staking registration
can be used.

The pre-staking registration procedure works as follows:

1. The BTC staker constructs the following transactions (whose specifications
   can be found [here](../../docs/staking-script.md)) and sends them on Babylon
   through the [MsgCreateBTCDelegation](#msgcreatebtcdelegation) message with the
   inclusion proof not set:
    - an unsigned _staking transaction_ committing their funds.
    - a pre-signed _slashing transaction_ that can spend the staking transaction once the
      finality provider is slashed,
    - an unsigned _unbonding transaction_ that spends the staking transaction to start
      the early unbonding process, and
    - a pre-signed _unbonding slashing transaction_ that can spend the unbonding
      transaction once the finality provider is slashed. The BTC staker
      pre-signs the slashing transaction and unbonding slashing transaction.
2. The covenant committee verifies the above transactions and add the
   required signatures for the slashing transactions.
3. The BTC staker views the above confirmation and can now feel confident in submitting
   the transaction to Bitcoin.
4. Once the transaction is on Bitcoin and with sufficient confirmations
   the staker can send a
   [MsgAddBTCDelegationInclusionProof](#msgaddbtcdelegationinclusionproof)
   with the inclusion proof for the stake receiving sufficient confirmations.
   The stake is now active. In the case the staker does not monitor the Bitcoin
   ledger for confirmation, the [Vigilante BTC Staking Tracker](https://github.com/babylonlabs-io/vigilante)
   will pick it up and submit the inclusion proof to Babylon.
 
### Voting Power and Finality

Babylon's [Finality module](../finality) will make use of the voting power table
maintained in the BTC Staking module to determine the finalization status of
each block, identify equivocations of finality providers, and slash BTC
delegations under culpable finality providers.

### On-Demand Unbonding Tracking

A BTC staker can unbond early by signing the unbonding transaction and
submitting it to Bitcoin. The BTC Staking module identifies unbonding requests
through this signature reported by the [BTC staking tracker
daemon](https://github.com/babylonlabs-io/vigilante), and will consider the BTC
delegation unbonded immediately upon such a signature.

## States

The BTC Staking module uses the following prefixed namespaces within its KV store to organize different types of data

### Parameters

The [parameter management](./keeper/params.go) maintains the BTC Staking module's
parameters. The BTC Staking module's parameters are represented as a `Params`
[object](../../proto/babylon/btcstaking/v1/params.proto) defined as follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.goproto_stringer) = false;
  // PARAMETERS COVERING STAKING
  // covenant_pks is the list of public keys held by the covenant committee
  // each PK follows encoding in BIP-340 spec on Bitcoin
  repeated bytes covenant_pks = 1
      [ (gogoproto.customtype) =
            "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // covenant_quorum is the minimum number of signatures needed for the covenant
  // multisignature
  uint32 covenant_quorum = 2;
  // min_staking_value_sat is the minimum of satoshis locked in staking output
  int64 min_staking_value_sat = 3;
  // max_staking_value_sat is the maximum of satoshis locked in staking output
  int64 max_staking_value_sat = 4;
  // min_staking_time is the minimum lock time specified in staking output
  // script
  uint32 min_staking_time_blocks = 5;
  // max_staking_time_blocks is the maximum lock time time specified in staking
  // output script
  uint32 max_staking_time_blocks = 6;
  // PARAMETERS COVERING SLASHING
  // slashing_pk_script is the pk_script expected in slashing output ie. the
  // first output of slashing transaction
  bytes slashing_pk_script = 7;
  // min_slashing_tx_fee_sat is the minimum amount of tx fee (quantified
  // in Satoshi) needed for the pre-signed slashing tx. It covers both:
  // staking slashing transaction and unbonding slashing transaction
  int64 min_slashing_tx_fee_sat = 8;
  // slashing_rate determines the portion of the staked amount to be slashed,
  // expressed as a decimal (e.g., 0.5 for 50%). Maximal precion is 2 decimal
  // places
  string slashing_rate = 9 [
    (cosmos_proto.scalar) = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable) = false
  ];
  // PARAMETERS COVERING UNBONDING
  // unbonding_time is the exact unbonding time required from unbonding
  // transaction it must be larger than `checkpoint_finalization_timeout` from
  // `btccheckpoint` module
  uint32 unbonding_time_blocks = 10;
  // unbonding_fee exact fee required for unbonding transaction
  int64 unbonding_fee_sat = 11;
  // PARAMETERS COVERING FINALITY PROVIDERS
  // min_commission_rate is the chain-wide minimum commission rate that a
  // finality provider can charge their delegators expressed as a decimal (e.g.,
  // 0.5 for 50%). Maximal precion is 2 decimal places
  string min_commission_rate = 12 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable) = false
  ];
  // base gas fee for delegation creation
  uint64 delegation_creation_base_gas_fee = 13;
  // allow_list_expiration_height is the height at which the allow list expires
  // i.e all staking transactions are allowed to enter Babylon chain afterwards
  // setting it to 0 means allow list is disabled
  uint64 allow_list_expiration_height = 14;
  // btc_activation_height is the btc height from which parameters are activated
  // (inclusive)
  uint32 btc_activation_height = 15;
}
```

### Finality providers

The [finality provider management](./keeper/finality_providers.go) maintains all
finality providers. The key is the finality provider's Bitcoin Secp256k1 public
key in [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
format, and the value is a `FinalityProvider`
[object](../../proto/babylon/btcstaking/v1/btcstaking.proto) representing a
finality provider.

```protobuf
// FinalityProvider defines a finality provider
message FinalityProvider {
  // addr is the bech32 address identifier of the finality provider.
  string addr = 1 [ (cosmos_proto.scalar) = "cosmos.AddressString" ];
  // description defines the description terms for the finality provider.
  cosmos.staking.v1beta1.Description description = 2;
  // commission defines the commission rate of the finality provider.
  string commission = 3 [
    (cosmos_proto.scalar) = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec"
  ];
  // btc_pk is the Bitcoin secp256k1 PK of this finality provider
  // the PK follows encoding in BIP-340 spec
  bytes btc_pk = 4
      [ (gogoproto.customtype) =
            "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // pop is the proof of possession of the btc_pk, where the BTC
  // private key signs the bech32 bbn addr of the finality provider.
  ProofOfPossessionBTC pop = 5;
  // slashed_babylon_height indicates the Babylon height when
  // the finality provider is slashed.
  // if it's 0 then the finality provider is not slashed
  uint64 slashed_babylon_height = 6;
  // slashed_btc_height indicates the BTC height when
  // the finality provider is slashed.
  // if it's 0 then the finality provider is not slashed
  uint32 slashed_btc_height = 7;
  // jailed defines whether the finality provider is jailed
  bool jailed = 8;
  // highest_voted_height is the highest height for which the
  // finality provider has voted
  uint32 highest_voted_height = 9;
  // consumer_id is the ID of the consumer the finality provider is operating
  // on. If it's missing / empty, it's assumed the finality provider is
  // operating in the Babylon chain.
  string consumer_id = 10;
  // commission_info contains information details of the finality provider commission.
  CommissionInfo commission_info = 11;
}
```

### BTC delegations

The [BTC delegation management](./keeper/btc_delegations.go) maintains all BTC
delegations. The key is the staking transaction hash corresponding to the BTC
delegation, and the value is a `BTCDelegation` object. The `BTCDelegation`
[structure](../../proto/babylon/btcstaking/v1/btcstaking.proto) includes
information of a BTC delegation and a structure `BTCUndelegation` that includes
information of its early unbonding path. The staking transaction's hash uniquely
identifies a `BTCDelegation` as creating a BTC delegation requires the staker to
submit a staking transaction to Bitcoin.

```protobuf
// BTCDelegation defines a BTC delegation
message BTCDelegation {
    // staker_addr is the address to receive rewards from BTC delegation.
    string staker_addr = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
    // btc_pk is the Bitcoin secp256k1 PK of this BTC delegation
    // the PK follows encoding in BIP-340 spec
    bytes btc_pk = 2 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
    // pop is the proof of possession of babylon_pk and btc_pk
    ProofOfPossessionBTC pop = 3;
    // fp_btc_pk_list is the list of BIP-340 PKs of the finality providers that
    // this BTC delegation delegates to
    // If there is more than 1 PKs, then this means the delegation is restaked
    // to multiple finality providers
    repeated bytes fp_btc_pk_list = 4 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
    // staking_time is the number of blocks for which the delegation is locked on BTC chain
    uint32 staking_time = 5;
    // start_height is the start BTC height of the BTC delegation
    // it is the start BTC height of the timelock
    uint32 start_height = 6;
    // end_height is the end height of the BTC delegation
    // it is calculated by end_height = start_height + staking_time
    uint32 end_height = 7;
    // total_sat is the total amount of BTC stakes in this delegation
    // quantified in satoshi
    uint64 total_sat = 8;
    // staking_tx is the staking tx
    bytes staking_tx  = 9;
    // staking_output_idx is the index of the staking output in the staking tx
    uint32 staking_output_idx = 10;
    // slashing_tx is the slashing tx
    // It is partially signed by SK corresponding to btc_pk, but not signed by
    // finality provider or covenant yet.
    bytes slashing_tx = 11 [ (gogoproto.customtype) = "BTCSlashingTx" ];
    // delegator_sig is the signature on the slashing tx
    // by the delegator (i.e., SK corresponding to btc_pk).
    // It will be a part of the witness for the staking tx output.
    bytes delegator_sig = 12 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
    // covenant_sigs is a list of adaptor signatures on the slashing tx
    // by each covenant member
    // It will be a part of the witness for the staking tx output.
    repeated CovenantAdaptorSignatures covenant_sigs = 13;
    // unbonding_time describes how long the funds will be locked either in unbonding output
    // or slashing change output
    uint32 unbonding_time = 14;
    // btc_undelegation is the information about the early unbonding path of the BTC delegation
    BTCUndelegation btc_undelegation = 15;
    // version of the params used to validate the delegation
    uint32 params_version = 16;
}

// DelegatorUnbondingInfo contains the information about transaction which spent
// the staking output. It contains:
// - spend_stake_tx: the transaction which spent the staking output
// - spend_stake_tx_inclusion_block_hash: the block hash of the block in which
// spend_stake_tx was included
// - spend_stake_tx_sig_inclusion_index: the index of spend_stake_tx in the block
message DelegatorUnbondingInfo {
    // spend_stake_tx is the transaction which spent the staking output. It is
    // filled only if spend_stake_tx is different than unbonding_tx registered
    // on the Babylon chain.
    bytes spend_stake_tx = 1;
}

// BTCUndelegation contains the information about the early unbonding path of the BTC delegation
message BTCUndelegation {
    // unbonding_tx is the transaction which will transfer the funds from staking
    // output to unbonding output. Unbonding output will usually have lower timelock
    // than staking output.
    bytes unbonding_tx = 1;
    // slashing_tx is the slashing tx for unbonding transactions
    // It is partially signed by SK corresponding to btc_pk, but not signed by
    // finality provider or covenant yet.
    bytes slashing_tx = 2 [ (gogoproto.customtype) = "BTCSlashingTx" ];
    // delegator_slashing_sig is the signature on the slashing tx
    // by the delegator (i.e., SK corresponding to btc_pk).
    // It will be a part of the witness for the unbonding tx output.
    bytes delegator_slashing_sig = 3 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
    // covenant_slashing_sigs is a list of adaptor signatures on the slashing tx
    // by each covenant member
    // It will be a part of the witness for the staking tx output.
    repeated CovenantAdaptorSignatures covenant_slashing_sigs = 4;
    // covenant_unbonding_sig_list is the list of signatures on the unbonding tx
    // by covenant members
    // It must be provided after processing undelegate message by Babylon
    repeated SignatureInfo covenant_unbonding_sig_list = 5;
    // delegator_unbonding_info is the information about transaction which spent
    // the staking output
    DelegatorUnbondingInfo delegator_unbonding_info = 6;
}
```

### BTC delegation index

The [BTC delegation index management](./keeper/btc_delegators.go) maintains an
index between the BTC delegator and its BTC delegations. The key is the BTC
delegator's Bitcoin secp256k1 public key in BIP-340 format, and the value is a
`BTCDelegatorDelegationIndex`
[object](../../proto/babylon/btcstaking/v1/btcstaking.proto) that contains
staking transaction hashes of the delegator's BTC delegations.

```protobuf
// BTCDelegatorDelegationIndex is a list of staking tx hashes of BTC delegations from the same delegator.
message BTCDelegatorDelegationIndex {
   repeated bytes staking_tx_hash_list = 1;
}
```

## Messages

The BTC Staking module handles the following messages from finality providers,
BTC stakers (aka delegators), and covenant emulators. The message formats are
defined at
[proto/babylon/btcstaking/v1/tx.proto](../../proto/babylon/btcstaking/v1/tx.proto).
The message handlers are defined at
[x/btcstaking/keeper/msg_server.go](./keeper/msg_server.go). For more information on the SDK messages, refer to the [Cosmos SDK documentation on messages and queries](https://docs.cosmos.network/main/build/building-modules/messages-and-queries)

### MsgCreateFinalityProvider

The `MsgCreateFinalityProvider` message is used for creating a finality
provider. It is typically submitted by a finality provider via the [finality
provider](https://github.com/babylonlabs-io/finality-provider) program.

```protobuf
// MsgCreateFinalityProvider is the message for creating a finality provider
message MsgCreateFinalityProvider {
  option (cosmos.msg.v1.signer) = "addr";
  // addr defines the address of the finality provider that will receive
  // the commissions to all the delegations.
  string addr = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  // description defines the description terms for the finality provider
  cosmos.staking.v1beta1.Description description = 2;
  // commission defines the commission rate of the finality provider
  string commission = 3 [
    (cosmos_proto.scalar)  = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec"
  ];
  // btc_pk is the Bitcoin secp256k1 PK of this finality provider
  // the PK follows encoding in BIP-340 spec
  bytes btc_pk = 4 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // pop is the proof of possession of btc_pk over the FP signer address.
  ProofOfPossessionBTC pop = 5;
}
```

where `Description` is adapted from Cosmos SDK's staking module and is defined
as follows:

```protobuf
// Description defines a validator description.
message Description {
  option (gogoproto.equal) = true;

  // moniker defines a human-readable name for the validator.
  string moniker = 1;
  // identity defines an optional identity signature (ex. UPort or Keybase).
  string identity = 2;
  // website defines an optional website link.
  string website = 3;
  // security_contact defines an optional email for security contact.
  string security_contact = 4;
  // details define other optional details.
  string details = 5;
}
```

Upon `MsgCreateFinalityProvider`, a Babylon node will execute as follows:

1. Verify a [proof of
   possession](https://rist.tech.cornell.edu/papers/pkreg.pdf) indicating the
   ownership of the Bitcoin secret keys over the Babylon address.
2. Ensure the given commission rate is at least the `MinCommissionRate` in the
   parameters and at most 100%.
3. Ensure the finality provider does not exist already.
4. Ensure the finality provider is not slashed.
5. Create a `FinalityProvider` object and save it to finality provider storage.

### MsgEditFinalityProvider

The `MsgEditFinalityProvider` message is used for editing the information of an
existing finality provider, including the commission and the description. It
needs to be submitted by using the Babylon account registered in the finality
provider.

```protobuf
// MsgEditFinalityProvider is the message for editing an existing finality provider
message MsgEditFinalityProvider {
  option (cosmos.msg.v1.signer) = "addr";
  // addr the address of the finality provider that whishes to edit his information.
  string addr = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  // btc_pk is the Bitcoin secp256k1 PK of the finality provider to be edited
  bytes btc_pk = 2;
  // description defines the updated description terms for the finality provider
  cosmos.staking.v1beta1.Description description = 3;
  // commission defines the updated commission rate of the finality provider
  string commission = 4 [
    (cosmos_proto.scalar)  = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec"
  ];
}
```

Upon `MsgEditFinalityProvider`, a Babylon node will execute as follows:

1. Validate the formats of the description.
2. Ensure the given commission rate is at least the `MinCommissionRate` in the
   parameters and at most 100%.
3. Get the finality provider with the given `btc_pk` from the finality provider
   storage.
4. Ensure the address `addr` matches to the address in the finality provider.
5. Change the `description` and `commission` in the finality provider to the
   values supplied in the message, and write back the finality provider to the
   finality provider storage.

### MsgCreateBTCDelegation

The `MsgCreateBTCDelegation` message is used for delegating some bitcoin to a
finality provider. It is typically submitted by a BTC delegator via the [BTC
staker](https://github.com/babylonlabs-io/btc-staker) program.

```protobuf
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

Upon `MsgCreateBTCDelegation`, a Babylon node will execute as follows:

1. Ensure the given unbonding time is larger than `max(MinUnbondingTime,
CheckpointFinalizationTimeout)`, where `MinUnbondingTime` and
   `CheckpointFinalizationTimeout` are module parameters from BTC Staking module
   and B
2. Verify the staking transaction and slashing transaction, including
    1. Ensure the information provided in the request is consistent with the
       staking transaction's BTC script.
    2. Ensure the staking transaction's timelock has more than
       `CheckpointFinalizationTimeout` BTC blocks left.
    3. Verify the Merkle proof of inclusion of the staking transaction against
       the BTC light client. <!-- TODO: add a  link to btccheckpoint doc -->
    4. Ensure the staking transaction and slashing transaction are valid and
       consistent, as per the [specification](../../docs/staking-script.md) of
       their formats.
    5. Verify the Schnorr signature on the slashing transaction signed by the BTC
       delegator.
3. Verify the unbonding transaction and unbonding slashing transaction,
   including
    1. Ensure the unbonding transaction's input points to the staking
       transaction.
    2. Verify the Schnorr signature on the slashing path of the unbonding
       transaction by the BTC delegator.
    3. Verify the unbonding transaction and the unbonding path's slashing
       transaction are valid and consistent, as per the
       [specification](../../docs/staking-script.md) of their formats.
4. Verify a [proof of
   possession](https://rist.tech.cornell.edu/papers/pkreg.pdf) indicating the
   ownership of the Bitcoin secret key over the Babylon staker address.
5. Ensure the staking transaction is not duplicated with an existing BTC
   delegation known to Babylon.
6. Ensure the finality providers that the bitcoin are delegated to are known to
   Babylon.
7. If the allow-list is enabled, ensure that the staking transaction is
   in the allow-list.
8. If the delegation contains an inclusion proof (optional due to the capability
    for pre-staking registration), verify the inclusion proof and ensure that it is 
   `BTCConfirmationDepth`-deep in the Bitcoin blockchain, where 
   `BTCConfirmationDepth` is a module parameter specified in the BTC
   Checkpoint module. <!-- TODO: add a  link to btccheckpoint doc -->
9. Create a `BTCDelegation` object and save it to the BTC delegation storage and
   the BTC delegation index storage.


### MsgAddBTCDelegationInclusionProof

The `MsgAddBTCDelegationInclusionProof` message is used for submitting
the proof of inclusion of a Bitcoin Stake delegation on the
Bitcoin blockchain.
This message is utilised for notifying the Babylon blockchain
that a staking transaction that was previously submitted through
the pre- staking registration process and is now on Bitcoin and has
received sufficient confirmations to become active.

```protobuf
// MsgAddBTCDelegationInclusionProof is the message for adding proof of inclusion of BTC delegation on BTC chain
message MsgAddBTCDelegationInclusionProof {
  option (cosmos.msg.v1.signer) = "signer";

  string signer = 1;
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 2;
  // staking_tx_inclusion_proof is the inclusion proof of the staking tx in BTC chain
  InclusionProof staking_tx_inclusion_proof = 3;
}
```

Upon `MsgAddBTCDelegationInclusionProof`, a Babylon
node will execute as follows:
1. Ensure that the staking transaction is tracked by Babylon.
2. Check whether the delegation has an inclusion proof. If yes, exit.
3. Check whether the delegation has not received a quorum of covenant sigs.
   If not, exist.
4. Check whether the delegation has been unbonded. If yes, exit.
5. Verify the inclusion proof in conjunction with the on-chain BTC light client.
6. Activate the transaction.

### MsgAddCovenantSigs

The `MsgAddCovenantSigs` message is used for submitting signatures on a BTC
delegation signed by a covenant committee member. It is typically submitted by a
covenant committee member via the [covenant
emulator](https://github.com/babylonlabs-io/covenant-emulator) program.

```protobuf
// MsgAddCovenantSigs is the message for handling signatures from a covenant member
message MsgAddCovenantSigs {
  option (cosmos.msg.v1.signer) = "signer";

  string signer = 1;
  // pk is the BTC public key of the covenant member
  bytes pk = 2  [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 3;
  // sigs is a list of adaptor signatures of the covenant
  // the order of sigs should respect the order of finality providers
  // of the corresponding delegation
  repeated bytes slashing_tx_sigs = 4;
  // unbonding_tx_sig is the signature of the covenant on the unbonding tx submitted to babylon
  // the signature follows encoding in BIP-340 spec
  bytes unbonding_tx_sig = 5 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
  // slashing_unbonding_tx_sigs is a list of adaptor signatures of the covenant
  // on slashing tx corresponding to unbonding tx submitted to babylon
  // the order of sigs should respect the order of finality providers
  // of the corresponding delegation
  repeated bytes slashing_unbonding_tx_sigs = 6;
}
```

Upon `AddCovenantSigs`, a Babylon node will execute as follows:

1. Ensure the given BTC delegation is known to Babylon.
2. Ensure the given covenant public key is in the covenant committee.
3. Verify each covenant adaptor signature on the slashing transaction. Note that
   each covenant adaptor signature is encrypted by a finality provider's BTC
   public key.
4. Verify the covenant Schnorr signature on the unbonding transactions.
5. Verify each covenant adaptor signature on the slashing transaction of the
   unbonding path.
6. Add the covenant signatures to the given `BTCDelegation` in the BTC
   delegation storage.

### MsgBTCUndelegate

The `MsgBTCUndelegate` message is used for unbonding bitcoin from a given
finality provider. It is typically reported by the [BTC staking
tracker](https://github.com/babylonlabs-io/vigilante/tree/main/btcstaking-tracker)
program which proactively monitors unbonding transactions on Bitcoin.

```protobuf
// MsgBTCUndelegate is the message for handling signature on unbonding tx
// from its delegator. This signature effectively proves that the delegator
// wants to unbond this BTC delegation
message MsgBTCUndelegate {
  option (cosmos.msg.v1.signer) = "signer";

  string signer = 1;
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 2;
  // stake_spending_tx is a bitcoin transaction that spends the staking
  // transaction i.e it has staking output as an input
  bytes stake_spending_tx = 3;
  // spend_spending_tx_inclusion_proof is the proof of inclusion of the
  // stake_spending_tx in the BTC chain
  InclusionProof stake_spending_tx_inclusion_proof = 4;
  // funding_transactions is a list of bitcoin transactions that funds the stake_spending_tx
  // i.e. they are inputs of the stake_spending_tx
  repeated bytes funding_transactions = 5;
}
```

Upon `BTCUndelegate`, a Babylon node will execute as follows:

1. Ensure the given BTC delegation is still active.
2. Verify the Schnorr signature on the unbonding transaction from the BTC
   delegator. If valid, this signature effectively proves that the BTC delegator
   wants to unbond this BTC delegation from Babylon.
3. Add the Schnorr signature to the `BTCDelegation` in the BTC delegation
   storage. Babylon will consider this BTC delegation to be unbonded from now
   on.

### MsgUpdateParams

The `MsgUpdateParams` message is used for updating the module parameters for the
BTC Staking module. It can only be executed via a govenance proposal.

```protobuf
// MsgUpdateParams defines a message for updating btcstaking module parameters.
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  // authority is the address of the governance account.
  // just FYI: cosmos.AddressString marks that this field should use type alias
  // for AddressString instead of string, but the functionality is not yet implemented
  // in cosmos-proto
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];

  // params defines the finality parameters to update.
  //
  // NOTE: All parameters must be supplied.
  Params params = 2 [(gogoproto.nullable) = false];
}
```

### MsgSelectiveSlashingEvidence

The `MsgSelectiveSlashingEvidence` message is used for submitting evidences for
selective slashing offences. In a selective slashing offence, the adversarial
finality provider chooses a victim BTC delegation, signs its slashing
transaction, and decrypts covenant adaptor signatures to the Schnorr signatures
using its secret key, before submitting this slashing transaction to Bitcoin. By
observing a pair of a [Schnorr
signature](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki) and
an [adaptor signature](https://bitcoinops.org/en/topics/adaptor-signatures/)
from the covenant committee, anyone can extract the finality provider's secret
key due to the [adaptor signature
properties](../../crypto/schnorr-adaptor-signature/README.md).

```proto
// MsgSelectiveSlashingEvidence is the message for handling evidence of selective slashing
// launched by a finality provider
message MsgSelectiveSlashingEvidence {
  option (cosmos.msg.v1.signer) = "signer";

  string signer = 1;
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 2;
  // recovered_fp_btc_sk is the BTC SK of the finality provider who
  // launches the selective slashing offence. The SK is recovered by
  // using a covenant adaptor signature and the corresponding Schnorr
  // signature
  bytes recovered_fp_btc_sk = 3;
}
```

Upon `MsgSelectiveSlashingEvidence`, a Babylon node will execute as follows:

1. Find the BTC delegation with the given staking transaction hash.
2. Ensure the BTC delegation is active or unbonding.
3. Ensure the given secret key corresponds to the finality provider's public
   key.
4. At this point, the finality provider must have done selective slashing. Thus,
   slash the finality provider and emit an event `EventSelectiveSlashing` about
   this.

The `MsgSelectiveSlashingEvidence` is typically reported by the [BTC staking
tracker](https://github.com/babylonlabs-io/vigilante/tree/main/btcstaking-tracker)
program. It keeps monitoring for slashing transactions on Bitcoin. Upon each
slashing transaction, it will try to extract the finality provider's secret key.
If successful, it will construct a `MsgSelectiveSlashingEvidence` message and
submit it to Babylon.

## BeginBlocker

Upon `BeginBlock`, the BTC Staking module will index the current BTC tip height. This will be used for determining the status of BTC delegations.

The logic is defined at [x/btcstaking/abci.go](./abci.go).

## Events

The BTC staking module emits a set of events for external subscribers. The events are defined
at `proto/babylon/btcstaking/v1/events.proto`.

### Finality provider events

```protobuf
// EventFinalityProviderCreated is the event emitted when a finality provider is created
message EventFinalityProviderCreated {
  // btc_pk_hex is the hex string of Bitcoin secp256k1 PK of this finality provider
  string btc_pk_hex = 1;
  // addr is the babylon address to receive commission from delegations.
  string addr = 2;
  // commission defines the commission rate of the finality provider in decimals.
  string commission = 3;
  // moniker defines a human-readable name for the finality provider.
  string moniker = 4;
  // identity defines an optional identity signature (ex. UPort or Keybase).
  string identity = 5;
  // website defines an optional website link.
  string website = 6;
  // security_contact defines an optional email for security contact.
  string security_contact = 7;
  // details define other optional details.
  string details = 8;
}

// EventFinalityProviderEdited is the event emitted when a finality provider is edited
message EventFinalityProviderEdited {
  // btc_pk_hex is the hex string of Bitcoin secp256k1 PK of this finality provider
  string btc_pk_hex = 1;
  // commission defines the commission rate of the finality provider in decimals.
  string commission = 2;
  // moniker defines a human-readable name for the finality provider.
  string moniker = 3;
  // identity defines an optional identity signature (ex. UPort or Keybase).
  string identity = 4;
  // website defines an optional website link.
  string website = 5;
  // security_contact defines an optional email for security contact.
  string security_contact = 6;
  // details define other optional details.
  string details = 7;
}

// A finality provider starts with status INACTIVE once registered.
// Possible status transitions are when:
// 1. it has accumulated sufficient delegations and has
// timestamped public randomness:
// INACTIVE -> ACTIVE
// 2. it is jailed due to downtime:
// ACTIVE -> JAILED
// 3. it is slashed due to double-sign:
// ACTIVE -> SLASHED
// 4. it is unjailed after a jailing period:
// JAILED -> INACTIVE/ACTIVE (depending on (1))
// 5. it does not have sufficient delegations or does not
// have timestamped public randomness:
// ACTIVE -> INACTIVE.
// Note that it is impossible for a SLASHED finality provider to
// transition to other status
message EventFinalityProviderStatusChange {
  // btc_pk is the BTC public key of the finality provider
  string btc_pk = 1;
  // new_status is the status that the finality provider
  // is transitioned to, following FinalityProviderStatus
  string new_state = 2;
}
```

### Delegation events

```protobuf

// EventBTCDelegationCreated is the event emitted when a BTC delegation is created
// on the Babylon chain
message EventBTCDelegationCreated {
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 1;
  // version of the params used to validate the delegation
  string params_version = 2;
  // finality_provider_btc_pks_hex is the list of hex str of Bitcoin secp256k1 PK of
  // the finality providers that this BTC delegation delegates to
  // the PK follows encoding in BIP-340 spec
  repeated string finality_provider_btc_pks_hex = 3;
  // staker_btc_pk_hex is the hex str of Bitcoin secp256k1 PK of the staker that
  // creates this BTC delegation the PK follows encoding in BIP-340 spec
  string staker_btc_pk_hex = 4;
  // staking_time is the timelock of the staking tx specified in the BTC script
  string staking_time = 5;
  // staking_amount is the total amount of BTC stake in this delegation
  // quantified in satoshi
  string staking_amount = 6;
  // unbonding_time is the time is timelock on unbonding tx chosen by the staker
  string unbonding_time = 7;
  // unbonding_tx is hex encoded bytes of the unsigned unbonding tx
  string unbonding_tx = 8;
  // new_state of the BTC delegation
  string new_state = 9;
}

// EventCovenantSignatureReceived is the event emitted when a covenant committee
// sends valid covenant signatures for a BTC delegation
message EventCovenantSignatureReceived{
  // staking_tx_hash is the hash of the staking identifing the BTC delegation
  // that this covenant signature is for
  string staking_tx_hash = 1;
  // covenant_btc_pk_hex is the hex str of Bitcoin secp256k1 PK of the
  // covnenat committee that send the signature
  string covenant_btc_pk_hex = 2;
  // covenant_unbonding_signature_hex is the hex str of the BIP340 Schnorr
  // signature of the covenant committee on the unbonding tx
  string covenant_unbonding_signature_hex = 3;
}

// EventCovenantQuorumReached is the event emitted quorum of covenant committee
// is reached for a BTC delegation
message EventCovenantQuorumReached {
  // staking_tx_hash is the hash of the staking identifing the BTC delegation
  // that this covenant signature is for
  string staking_tx_hash = 1;
  // new_state of the BTC delegation
  string new_state = 2;
}

// EventBTCDelegationInclusionProofReceived is the event emitted when a BTC delegation
// inclusion proof is received
message EventBTCDelegationInclusionProofReceived {
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 1;
  // start_height is the start BTC height of the BTC delegation
  // it is the start BTC height of the timelock
  string start_height = 2;
  // end_height is the end height of the BTC delegation
  // it is calculated by end_height = start_height + staking_time
  string end_height = 3;
  // new_state of the BTC delegation
  string new_state = 4;
}

// EventBTCDelgationUnbondedEarly is the event emitted when a BTC delegation
// is unbonded by staker sending unbonding tx to BTC
message EventBTCDelgationUnbondedEarly {
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 1;
  // new_state of the BTC delegation
  string new_state = 2;
}

// EventBTCDelegationExpired is the event emitted when a BTC delegation
// is unbonded by expiration of the staking tx timelock
message EventBTCDelegationExpired {
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 1;
  // new_state of the BTC delegation
  string new_state = 2;
}
```

## Queries

The BTC Staking module provides a set of queries related to the status of finality providers, BTC delegations, and other staking-related data. These queries can be accessed via gRPC and REST endpoints.

Available Queries:
Parameters
Endpoint: `/babylon/btcstaking/v1/params`
Description: Queries the current parameters of the BTC Staking module.

Params by Version
Endpoint: `/babylon/btcstaking/v1/params/{version}`
Description: Queries the parameters of the module for a specific past version.

Finality Providers
Endpoint: `/babylon/btcstaking/v1/finality_providers`
Description: Retrieves all finality providers in the Babylon staking module.

Finality Provider by Public Key
Endpoint: `/babylon/btcstaking/v1/finality_providers/{fp_btc_pk_hex}/finality_provider`
Description: Retrieves information about a specific finality provider by its Bitcoin public key (in BIP-340 format).

BTC Delegations by Status
Endpoint: `/babylon/btcstaking/v1/btc_delegations/{status}`
Description: Queries all BTC delegations under a given status.

Active Finality Providers at Height
Endpoint: `/babylon/btcstaking/v1/finality_providers/{height}`
Description: Retrieves finality providers with non-zero voting power at a specific Babylon block height.

Finality Provider Power at Height
Endpoint: `/babylon/btcstaking/v1/finality_providers/{fp_btc_pk_hex}/power/`{height}
Description: Queries the voting power of a specific finality provider at a given Babylon block height.

Finality Provider Current Power
Endpoint: `/babylon/btcstaking/v1/finality_providers/{fp_btc_pk_hex}/power`
Description: Queries the current voting power of a specific finality provider.

Activated Height
Endpoint: `/babylon/btcstaking/v1/activated_height`
Description: Queries the block height when the BTC staking protocol was activated (i.e., the first height when there existed at least one finality provider with voting power).

Finality Provider Delegations
Endpoint: `/babylon/btcstaking/v1/finality_providers/{fp_btc_pk_hex}/delegations`
Description: Queries all BTC delegations under a specific finality provider.

BTC Delegation by Staking Transaction Hash
Endpoint: `/babylon/btcstaking/v1/btc_delegation/{staking_tx_hash_hex}`
Description: Retrieves a specific BTC delegation by its corresponding staking transaction hash.

Additional Information:
For further details on how to use these queries and additional documentation, please refer to docs.babylonlabs.io.

<!-- TODO: update Babylon doc website -->
