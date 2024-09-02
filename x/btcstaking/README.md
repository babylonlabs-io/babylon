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
- [States](#states)
  - [Parameters](#parameters)
  - [Finality providers](#finality-providers)
  - [BTC delegations](#btc-delegations)
  - [BTC delegation index](#btc-delegation-index)
  - [Voting power table](#voting-power-table)
  - [Params](#params)
- [Messages](#messages)
  - [MsgCreateFinalityProvider](#msgcreatefinalityprovider)
  - [MsgEditFinalityProvider](#msgeditfinalityprovider)
  - [MsgCreateBTCDelegation](#msgcreatebtcdelegation)
  - [MsgAddCovenantSigs](#msgaddcovenantsigs)
  - [MsgBTCUndelegate](#msgbtcundelegate)
  - [MsgUpdateParams](#msgupdateparams)
  - [MsgSelectiveSlashingEvidence](#msgselectiveslashingevidence)
- [BeginBlocker](#beginblocker)
- [Events](#events)
- [Queries](#queries)

## Concepts

Babylon's Bitcoin Staking protocol allows bitcoin holders to _trustlessly_ stake
their bitcoins for providing economic security to the Babylon chain and other
Proof-of-Stake (PoS) blockchains, _without bridging their bitcoins elsewhere_.
The protocol consists of the following participants:

- **BTC staker (aka delegator)** who delegates their bitcoins to a finality
  provider in order to obtain staking reward.
- **Finality provider** who receives bitcoin delegations and participates in the
  _finality vote round_ on top of the CometBFT consensus.
- **Covenant emulation committee** who serves as the
  [covenants](https://covenants.info) to enforce spending conditions on bitcoins
  staked on Babylon.

The BTC Staking module is a major component in Babylon's BTC Staking protocol.
At a high level, the participants interact with the BTC Staking module as
follows:

1. A finality provider registers itself on the BTC Staking module.
2. A BTC staker delegates some bitcoins to the finality provider. This involves
   the following steps:
   1. The BTC staker submits a _staking transaction_ to Bitcoin. The staking
      transaction locks its bitcoins for a long period of time and specifies
      slashing conditions.
   2. The BTC staker constructs the following transactions (whose specifications
      can be found [here](../../docs/staking-script.md)):
      - a _slashing transaction_ that can spend the staking transaction once the
        finality provider is slashed,
      - an _unbonding transaction_ that spends the staking transaction to start
        the early unbonding process, and
      - an _unbonding slashing transaction_ that can spend the unbonding
        transaction once the finality provider is slashed. The BTC staker
        pre-signs the slashing transaction and unbonding slashing transaction.
   3. Once the staking transaction is confirmed on Bitcoin, the BTC staker sends
      the staking transaction, its inclusion proof, slashing transaction,
      unbonding transaction, and unbonding slashing transaction to Babylon.
3. The covenant committee verifies spending conditions of the staking
   transaction, and submits its signatures on the BTC staker's transactions. At
   this point, the finality provider receives voting power
   from the BTC delegation.
4. Upon each new block, the BTC Staking module will record the voting power
   table of finality providers.

Babylon's [Finality module](../finality) will make use of the voting power table
maintained in the BTC Staking module to determine the finalization status of
each block, identify equivocations of finality providers, and slash BTC
delegations under culpable finality providers.

A BTC staker can unbond early by signing the unbonding transaction and
submitting it to Bitcoin. The BTC Staking module identifies unbonding requests
through this signature reported by the [BTC staking tracker
daemon](https://github.com/babylonchain/vigilante), and will consider the BTC
delegation unbonded immediately upon such a signature.

## States

The BTC Staking module maintains the following KV stores.

### Parameters

The [parameter storage](./keeper/params.go) maintains the BTC Staking module's
parameters. The BTC Staking module's parameters are represented as a `Params`
[object](../../proto/babylon/btcstaking/v1/params.proto) defined as follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.goproto_stringer) = false;

  // covenant_pks is the list of public keys held by the covenant committee
  // each PK follows encoding in BIP-340 spec on Bitcoin
  repeated bytes covenant_pks = 1 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // covenant_quorum is the minimum number of signatures needed for the covenant
  // multisignature
  uint32 covenant_quorum = 2;
  // slashing address is the address that the slashed BTC goes to
  // the address is in string on Bitcoin
  string slashing_address = 3;
  // min_slashing_tx_fee_sat is the minimum amount of tx fee (quantified
  // in Satoshi) needed for the pre-signed slashing tx
  int64 min_slashing_tx_fee_sat = 4;
  // min_commission_rate is the chain-wide minimum commission rate that a finality provider can charge their delegators
  string min_commission_rate = 5 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  // slashing_rate determines the portion of the staked amount to be slashed,
  // expressed as a decimal (e.g., 0.5 for 50%).
  string slashing_rate = 6 [
      (cosmos_proto.scalar)  = "cosmos.Dec",
      (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
      (gogoproto.nullable)   = false
  ];
  // max_active_finality_providers is the maximum number of active finality providers in the BTC staking protocol
  uint32 max_active_finality_providers = 7;
  // min_unbonding_time is the minimum time for unbonding transaction timelock in BTC blocks
  uint32 min_unbonding_time = 8;

  // min_unbonding_rate is the minimum amount of BTC that are required in unbonding
  // output, expressed as a fraction of staking output
  // example: if min_unbonding_rate=0.9, then the unbonding output value
  // must be at least 90% of staking output, for staking request to be considered
  // valid
  string min_unbonding_rate = 9 [
    (cosmos_proto.scalar)  = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
}
```

### Finality providers

The [finality provider storage](./keeper/finality_providers.go) maintains all
finality providers. The key is the finality provider's Bitcoin Secp256k1 public
key in [BIP-340](https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki)
format, and the value is a `FinalityProvider`
[object](../../proto/babylon/btcstaking/v1/btcstaking.proto) representing a
finality provider.

```protobuf
// FinalityProvider defines a finality provider
message FinalityProvider {
   // addr is the bech32 address identifier of the finality provider.
   string addr = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
   // description defines the description terms for the finality provider.
   cosmos.staking.v1beta1.Description description = 2;
   // commission defines the commission rate of the finality provider.
   string commission = 3  [
      (cosmos_proto.scalar)  = "cosmos.Dec",
      (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec"
   ];
   // btc_pk is the Bitcoin secp256k1 PK of this finality provider
   // the PK follows encoding in BIP-340 spec
   bytes btc_pk = 4 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
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
   uint64 slashed_btc_height = 7;
}
```

### BTC delegations

The [BTC delegation storage](./keeper/btc_delegations.go) maintains all BTC
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
   // start_height is the start BTC height of the BTC delegation
   // it is the start BTC height of the timelock
   uint64 start_height = 5;
   // end_height is the end height of the BTC delegation
   // it is the end BTC height of the timelock - w
   uint64 end_height = 6;
   // total_sat is the total amount of BTC stakes in this delegation
   // quantified in satoshi
   uint64 total_sat = 7;
   // staking_tx is the staking tx
   bytes staking_tx  = 8;
   // staking_output_idx is the index of the staking output in the staking tx
   uint32 staking_output_idx = 9;
   // slashing_tx is the slashing tx
   // It is partially signed by SK corresponding to btc_pk, but not signed by
   // finality provider or covenant yet.
   bytes slashing_tx = 10 [ (gogoproto.customtype) = "BTCSlashingTx" ];
   // delegator_sig is the signature on the slashing tx
   // by the delegator (i.e., SK corresponding to btc_pk).
   // It will be a part of the witness for the staking tx output.
   bytes delegator_sig = 11 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
   // covenant_sigs is a list of adaptor signatures on the slashing tx
   // by each covenant member
   // It will be a part of the witness for the staking tx output.
   repeated CovenantAdaptorSignatures covenant_sigs = 12;
   // unbonding_time describes how long the funds will be locked either in unbonding output
   // or slashing change output
   uint32 unbonding_time = 13;
   // btc_undelegation is the information about the early unbonding path of the BTC delegation
   BTCUndelegation btc_undelegation = 14;
   // version of the params used to validate the delegation
   uint32 params_version = 15;
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
   // delegator_unbonding_sig is the signature on the unbonding tx
   // by the delegator (i.e., SK corresponding to btc_pk).
   // It effectively proves that the delegator wants to unbond and thus
   // Babylon will consider this BTC delegation unbonded. Delegator's BTC
   // on Bitcoin will be unbonded after timelock
   bytes delegator_unbonding_sig = 3 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
   // delegator_slashing_sig is the signature on the slashing tx
   // by the delegator (i.e., SK corresponding to btc_pk).
   // It will be a part of the witness for the unbonding tx output.
   bytes delegator_slashing_sig = 4 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
   // covenant_slashing_sigs is a list of adaptor signatures on the slashing tx
   // by each covenant member
   // It will be a part of the witness for the staking tx output.
   repeated CovenantAdaptorSignatures covenant_slashing_sigs = 5;
   // covenant_unbonding_sig_list is the list of signatures on the unbonding tx
   // by covenant members
   // It must be provided after processing undelegate message by Babylon
   repeated SignatureInfo covenant_unbonding_sig_list = 6;
}
```

### BTC delegation index

The [BTC delegation index storage](./keeper/btc_delegators.go) maintains an
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

### Voting power table

The [voting power table storage](./keeper/voting_power_table.go) maintains the
voting power table of all finality providers at each height of the Babylon
chain. The key is the block height concatenated with the finality provider's
Bitcoin secp256k1 public key in BIP-340 format, and the value is the finality
provider's voting power quantified in Satoshis.
Voting power is assigned to top `N` (defined in parameters) finality providers
that have BTC-timestamped public randomness for the height, ranked by the total
delegated value.

### Params

The [parameter storage](./keeper/params.go) maintains the parameters for the BTC
staking module.

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.goproto_stringer) = false;

  // covenant_pks is the list of public keys held by the covenant committee
  // each PK follows encoding in BIP-340 spec on Bitcoin
  repeated bytes covenant_pks = 1 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // covenant_quorum is the minimum number of signatures needed for the covenant
  // multisignature
  uint32 covenant_quorum = 2;
  // slashing address is the address that the slashed BTC goes to
  // the address is in string on Bitcoin
  string slashing_address = 3;
  // min_slashing_tx_fee_sat is the minimum amount of tx fee (quantified
  // in Satoshi) needed for the pre-signed slashing tx
  int64 min_slashing_tx_fee_sat = 4;
  // min_commission_rate is the chain-wide minimum commission rate that a finality provider can charge their delegators
  string min_commission_rate = 5 [
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false
  ];
  // slashing_rate determines the portion of the staked amount to be slashed,
  // expressed as a decimal (e.g., 0.5 for 50%).
  string slashing_rate = 6 [
      (cosmos_proto.scalar)  = "cosmos.Dec",
      (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
      (gogoproto.nullable)   = false
  ];
  // max_active_finality_providers is the maximum number of active finality providers in the BTC staking protocol
  uint32 max_active_finality_providers = 7;
  // min_unbonding_time is the minimum time for unbonding transaction timelock in BTC blocks
  uint32 min_unbonding_time = 8;
}
```

## Messages

The BTC Staking module handles the following messages from finality providers,
BTC stakers (aka delegators), and covenant emulators. The message formats are
defined at
[proto/babylon/btcstaking/v1/tx.proto](../../proto/babylon/btcstaking/v1/tx.proto).
The message handlers are defined at
[x/btcstaking/keeper/msg_server.go](./keeper/msg_server.go).

### MsgCreateFinalityProvider

The `MsgCreateFinalityProvider` message is used for creating a finality
provider. It is typically submitted by a finality provider via the [finality
provider](https://github.com/babylonchain/finality-provider) program.

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

The `MsgCreateBTCDelegation` message is used for delegating some bitcoins to a
finality provider. It is typically submitted by a BTC delegator via the [BTC
staker](https://github.com/babylonchain/btc-staker) program.

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
  // staking_tx is the staking tx along with the merkle proof of inclusion in btc block
  babylon.btccheckpoint.v1.TransactionInfo staking_tx = 7;
  // slashing_tx is the slashing tx
  // Note that the tx itself does not contain signatures, which are off-chain.
  bytes slashing_tx = 8 [ (gogoproto.customtype) = "BTCSlashingTx" ];
  // delegator_slashing_sig is the signature on the slashing tx by the delegator (i.e., SK corresponding to btc_pk).
  // It will be a part of the witness for the staking tx output.
  // The staking tx output further needs signatures from covenant and finality provider in
  // order to be spendable.
  bytes delegator_slashing_sig = 9 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
  // unbonding_time is the time lock used when funds are being unbonded. It is be used in:
  // - unbonding transaction, time lock spending path
  // - staking slashing transaction, change output
  // - unbonding slashing transaction, change output
  // It must be smaller than math.MaxUInt16 and larger that max(MinUnbondingTime, CheckpointFinalizationTimeout)
  uint32 unbonding_time = 10;
  // fields related to unbonding transaction
  // unbonding_tx is a bitcoin unbonding transaction i.e transaction that spends
  // staking output and sends it to the unbonding output
  bytes unbonding_tx = 11;
  // unbonding_value is amount of satoshis locked in unbonding output.
  // NOTE: staking_value and unbonding_value could be different because of the difference between the fee for staking tx and that for unbonding
  int64 unbonding_value = 12;
  // unbonding_slashing_tx is the slashing tx which slash unbonding contract
  // Note that the tx itself does not contain signatures, which are off-chain.
  bytes unbonding_slashing_tx = 13 [ (gogoproto.customtype) = "BTCSlashingTx" ];
  // delegator_unbonding_slashing_sig is the signature on the slashing tx by the delegator (i.e., SK corresponding to btc_pk).
  bytes delegator_unbonding_slashing_sig = 14 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
}
```

Upon `MsgCreateBTCDelegation`, a Babylon node will execute as follows:

1. Ensure the given unbonding time is larger than `max(MinUnbondingTime,
CheckpointFinalizationTimeout)`, where `MinUnbondingTime` and
   `CheckpointFinalizationTimeout` are module parameters from BTC Staking module
   and BTC Checkpoint module, respectively.
2. Verify a [proof of
   possession](https://rist.tech.cornell.edu/papers/pkreg.pdf) indicating the
   ownership of the Bitcoin secret key over the Babylon staker address.
3. Ensure the finality providers that the bitcoins are delegated to are known to
   Babylon.
4. Verify the staking transaction and slashing transaction, including
   1. Ensure the staking transaction is not duplicated with an existing BTC
      delegation known to Babylon.
   2. Ensure the information provided in the request is consistent with the
      staking transaction's BTC script.
   3. Ensure the staking transaction is `BTCConfirmationDepth`-deep in Bitcoin,
      where `BTCConfirmationDepth` is a module parameter specified in the BTC
      Checkpoint module. <!-- TODO: add a  link to btccheckpoint doc -->
   4. Ensure the staking transaction's timelock has more than
      `CheckpointFinalizationTimeout` BTC blocks left.
   5. Verify the Merkle proof of inclusion of the staking transaction against
      the BTC light client. <!-- TODO: add a  link to btccheckpoint doc -->
   6. Ensure the staking transaction and slashing transaction are valid and
      consistent, as per the [specification](../../docs/staking-script.md) of
      their formats.
   7. Verify the Schnorr signature on the slashing transaction signed by the BTC
      delegator.
5. Verify the unbonding transaction and unbonding slashing transaction,
   including
   1. Ensure the unbonding transaction's input points to the staking
      transaction.
   2. Verify the Schnorr signature on the slashing path of the unbonding
      transaction by the BTC delegator.
   3. Verify the unbonding transaction and the unbonding path's slashing
      transaction are valid and consistent, as per the
      [specification](../../docs/staking-script.md) of their formats.
6. Create a `BTCDelegation` object and save it to the BTC delegation storage and
   the BTC delegation index storage.

### MsgAddCovenantSigs

The `MsgAddCovenantSigs` message is used for submitting signatures on a BTC
delegation signed by a covenant committee member. It is typically submitted by a
covenant committee member via the [covenant
emulator](https://github.com/babylonchain/covenant-emulator) program.

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

The `MsgBTCUndelegate` message is used for unbonding bitcoins from a given
finality provider. It is typically reported by the [BTC staking
tracker](https://github.com/babylonchain/vigilante/tree/dev/btcstaking-tracker)
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
  // unbonding_tx_sig is the signature of the staker on the unbonding tx submitted to babylon
  // the signature follows encoding in BIP-340 spec
  bytes unbonding_tx_sig = 3 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
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
tracker](https://github.com/babylonchain/vigilante/tree/dev/btcstaking-tracker)
program. It keeps monitoring for slashing transactions on Bitcoin. Upon each
slashing transaction, it will try to extract the finality provider's secret key.
If successful, it will construct a `MsgSelectiveSlashingEvidence` message and
submit it to Babylon.

## BeginBlocker

Upon `BeginBlock`, the BTC Staking module will execute the following:

1. Index the current BTC tip height. This will be used for determining the
   status of BTC delegations.
2. Record the voting power table at the current height, by reconciling the
   voting power table at the last height with all events that affect voting
   power distribution (including newly active BTC delegations, newly unbonded
   BTC delegations, and slashed finality providers).
   Note that the voting power is assigned to a finality provider if it (1) has
   BTC-timestamped public randomness, and (2) it is ranked at top `N` by the
   total delegated value.
3. If the BTC Staking protocol is activated, i.e., there exists at least 1
   active BTC delegation, then record the reward distribution w.r.t. the active
   finality providers and active BTC delegations.

The logic is defined at [x/btcstaking/abci.go](./abci.go).

## Events

The BTC staking module emits a set of events as follows. The events are defined
at `proto/babylon/btcstaking/v1/events.proto`.

```protobuf
// EventNewFinalityProvider is the event emitted when a finality provider is created
message EventNewFinalityProvider { FinalityProvider fp = 1; }

// EventBTCDelegationStateUpdate is the event emitted when a BTC delegation's state is
// updated. There are the following possible state transitions:
// - non-existing -> pending, which happens upon `MsgCreateBTCDelegation`
// - pending -> active, which happens upon `MsgAddCovenantSigs`
// - active -> unbonded, which happens upon `MsgBTCUndelegate` or upon staking tx timelock expires
message EventBTCDelegationStateUpdate {
  // staking_tx_hash is the hash of the staking tx.
  // It uniquely identifies a BTC delegation
  string staking_tx_hash = 1;
  // new_state is the new state of this BTC delegation
  BTCDelegationStatus new_state = 2;
}

// EventSelectiveSlashing is the event emitted when an adversarial
// finality provider selectively slashes a BTC delegation. This will
// result in slashing of all BTC delegations under this finality provider.
message EventSelectiveSlashing {
  // evidence is the evidence of selective slashing
  SelectiveSlashingEvidence evidence = 1;
}

// EventPowerDistUpdate is an event that affects voting power distirbution
// of BTC staking protocol
message EventPowerDistUpdate {
  // EventSlashedFinalityProvider defines an event that a finality provider
  // is slashed
  // TODO: unify with existing slashing events
  message EventSlashedFinalityProvider {
    bytes pk = 1 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  }

  // ev is the event that affects voting power distribution
  oneof ev {
    // slashed_fp means a finality provider is slashed
    EventSlashedFinalityProvider slashed_fp = 1;
    // btc_del_state_update means a BTC delegation's state is updated
    EventBTCDelegationStateUpdate btc_del_state_update = 2;
  }
}
```

## Queries

The BTC staking module provides a set of queries about the status of finality
providers and BTC delegations, listed at
[docs.babylonchain.io](https://docs.babylonchain.io/docs/developer-guides/grpcrestapi#tag/BTCStaking).

<!-- TODO: update Babylon doc website -->
