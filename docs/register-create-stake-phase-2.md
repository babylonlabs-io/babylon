# Registering a phase-1 stake or creating a new stake

## Table of contents

- [Introduction](#introduction)
- [Babylon messages](#babylon-messages)
- [Stake creation/registraton](#stake-creationregistration)
- [On-demand unbonding](#on-demand-unbonding)
- [Withdrawing](#withdrawing)

## Introduction

<!-- Finish this section -->

This documentation guides you through the structures that need to be 
communicated to the Babylon chain in order to register a Phase-1 stake or 
create a new one.

We are going to start with the Babylon structures associated with BTC staking 
and how to construct them.

Then we are going to explore different use cases utilising the different 
structures.

    - vitalis note: maybe we can start with the use cases and put the structures 
    at the bottom

## Babylon messages

Babylon specifies a set of messages that are included inside transactions that 
are used for staking, unbonding, etc. We will go through the messages and 
how to construct them.

### MsgCreateBTCDelegation

The `MsgCreateBTCDelegation` message is used to create a BTC delegation. 
This message is sent to the Babylon chain to register a phase-1 stake or 
create a new stake.

The message contains the following fields:

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

We will go through the fields in the above message in detail to understand how 
to construct them.

### Staking Transaction

A staking transaction is a Bitcoin transaction that locks funds in a special 
staking output. The main entry point for creating a staking transaction is 
the `StakeFunds` function:

```go
func (app *App) StakeFunds(
    stakerAddress btcutil.Address,      // Address of the staker
    stakingAmount btcutil.Amount,       // Amount to stake
    fpPks []*btcec.PublicKey,          // Finality provider public keys
    stakingTimeBlocks uint16,          // How long to stake for
    sendToBabylonFirst bool,           // Pre/post approval flow
) (*chainhash.Hash, error)
```

To generate a staking transaction, you first need to build the staking information 
using `BuildStakingInfo`:

```go
stakingInfo, err := staking.BuildStakingInfo(
    stakerBtcPk,           // BTC public key
    fpBtcPks,             // finality provider public keys
    currentParams.CovenantPks,  // covenant committee public keys
    currentParams.CovenantQuruomThreshold,  // required number of covenant signatures
    stakingTime,          // how long to lock the BTC
    stakingValue,         // amount of BTC to stake
    network,              // Bitcoin network parameters
)
```

This staking information is used to create a Bitcoin transaction with a special 
output script that enforces the staking rules. The transaction is then created 
using:

```go
stakingTx, err := app.wc.CreateTransaction(
    []*wire.TxOut{stakingInfo.StakingOutput}, // staking output
    btcutil.Amount(feeRate),                  // transaction fee rate
    stakerAddress,                           // staker's address
    app.filterUtxoFnGen(),                   // UTXO filter (Unspent Transaction Output)
)
```

Once created, the transaction is broadcast to the Bitcoin network and its 
hash is returned. This hash (*chainhash.Hash) uniquely identifies the staking 
transaction and can be used to track its status or reference it in future 
transactions like unbonding.

### Unbonding Transaction

An unbonding transaction is a process initiated by a BTC staker to unlock and 
withdraw their stake before the originally committed timelock period has expired. 
It allows the staker to regain access to their locked funds earlier than planned.

#### Creating an Unbonding Transaction

To create an unbonding transaction, you will need to create the unbonding 
transaction using `createUndelegationData` function. You can find the function
in the [btc-staker/types.go](https://github.com/babylonchain/babylon/blob/main/btc-staker/types.go#L341) 
file. The function returns an `UnbondingSlashingDesc` containing:
- The unbonding transaction
- The unbonding value
- The timelock period
- The slashing transaction for the unbonding
- Spend information for the slashing path

This creates a complete unbonding transaction that can be submitted to the 
Babylon chain through a `MsgBTCUndelegate` message. The transaction will be 
monitored by the BTC staking tracker and requires covenant signatures before 
it can be executed on the Bitcoin network.

This will be what you can use `MsgCreateBTCDelegation` for registering phase-2 
stakes.

## Slashing Transactions

Slashing transactions are used to slash BTC stakers who violate the staking 
rules or the finality provider they have delegated to. These transactions 
ensure that malicious behavior can be penalized by taking a portion of the 
staker's funds according to the slashing rate.

The slashing transaction is initially created through the 
`slashingTxForStakingTx` function for the staking period, and then automatically 
generated through the `createUndelegationData` function when creating an 
unbonding transaction. These functions ensure the staker can be slashed during 
both the staking and unbonding periods. The generated transactions are included 
in the `MsgCreateBTCDelegation` message and require both staker and covenant 
signatures to be executed.

To see more information on how to generate the slashing transaction, see the 
[staker/types.go](https://github.com/babylonlabs-io/btc-staker/blob/main/staker/types.go#L111) file.

## Slashing Signatures

The slashing signatures are generated by the BTC staker and are used to sign 
the slashing transaction. The signatures are generated by the BTC staker and 
are used to sign the slashing transaction.  

The slashing signatures are generated by the BTC staker and are used to 
sign the slashing transaction. These signatures are generated during 
delegation creation and are required for both staking and unbonding transactions, 
setting up how funds will be distributed if slashing occurs.

To generate slashing signatures, the staker first creates a slashing 
transaction template using `slashingTxForStakingTx`. Then, using their private 
key, they sign this transaction with `SignTxWithOneScriptSpendInputFromScript` 
to create a Schnorr signature. This signature, along with signatures from a 
quorum of covenant members, is required to execute any slashing transaction. 
The signatures are verified using `VerifyTransactionSigWithOutput` to ensure 
they're valid before being stored with the delegation data.

```go
// 1. Create transaction structure
slashingTx, spendInfo, err := slashingTxForStakingTx(...)

// 2. Generate signatures (done separately)
signature, err := SignTxWithOneScriptSpendInputFromScript(
    slashingTx,          // transaction to sign
    fundingOutput,       // output being spent
    privateKey,          // private key for signing
    spendInfo.RevealedLeaf.Script,  // script being satisfied
)

// 3. Verify signatures
err = VerifyTransactionSigWithOutput(
    slashingTx,          // transaction to verify
    fundingOutput,       // output being spent
    script,              // script to check against
    pubKey,              // public key for verification
    signature,           // signature to verify
)
```

You can find more on the functions
in the [staker/types.go](https://github.com/babylonlabs-io/btc-staker/blob/main/staker/types.go) file.

## Inclusion Proof

An inclusion proof (also known as a Merkle proof) demonstrates that a specific 
Bitcoin transaction is included in a block. In the Babylon protocol, these 
proofs are optional when sending delegations to the Babylon network.

As shown in the code:

```go
type sendDelegationRequest struct {
    btcTxHash chainhash.Hash
    // optional field, if not provided, delegation will be sent to Babylon without
    // the inclusion proof
    inclusionInfo               *inclusionInfo
    requiredInclusionBlockDepth uint32
}
```

The inclusion proof is not automatically generated by the Babylon chain - it's an 
optional field in the delegation request. When provided, it contains the 
transaction index, block data, block height, and proof bytes in the 
`inclusionInfo` structure. This proof can be included in the 
`MsgCreateBTCDelegation` message when creating a delegation, but delegations 
can also be sent without a proof.

The inclusion proof follows two possible flows:

**Without proof:** Send delegation to Babylon first, then submit to Bitcoin later

**With proof:** Submit to Bitcoin first, wait for confirmations, then send 
delegation to Babylon with the inclusion proof.

To see more information about inclusion proofs, see the 
[staker/babylontypes.go](https://github.com/babylonlabs-io/btc-staker/blob/main/staker/babylontypes.go) file.

## Proof of Possession

Proof of Possession (PoP) is a signature that proves you own and control the 
BTC private key used for staking. This proof is a required component when 
creating a BTC delegation through `MsgCreateBTCDelegation`.
<!-- what specific messages -->

To generate a Proof of Possession, you need to sign a specific message using 
your BTC private key:

```go
pop, err := btcstaking.SignProofOfPossession(
    privateKey,      // Your BTC private key
    babylon_address, // Your Babylon address
    net,            // Network parameters
)
```
It is then returned as a `ProofOfPossessionBTC` message.

The PoP serves as a security measure in the Babylon protocol, ensuring that only 
the rightful owner of the BTC private key can create delegations. This 
prevents unauthorized delegations and is a crucial part of the staking security 
process.

## Other General Fields
When creating a BTC delegation, several general fields are required in addition to the signatures and proofs:

Staking Amount:
```go
stakingValue := btcutil.Amount(1000000) // Amount in satoshis
```

Version Parameters:
```go
// Get current parameters version from Babylon chain
params, err := babylonClient.GetBTCStakingParams(ctx)
if err != nil {
    return err
}

stakingTime := uint16(1000)    // Blocks for staking period
unbondingTime := uint16(100)   // Blocks for unbonding period
```

Fees:
```go   
stakingFee := btcutil.Amount(1000)    // Fee for staking transaction
slashingFee := btcutil.Amount(1000)   // Fee for slashing transaction
unbondingFee := btcutil.Amount(1000)  // Fee for unbonding transaction
```

Slashing rate:
```go
slashingRate := float64(0.1) // 10% slashing rate
```

### MsgAddBTCDelegationInclusionProof

The `MsgAddBTCDelegationInclusionProof` message is used for submitting
the proof of inclusion of a Bitcoin Stake delegation on the
Bitcoin blockchain.
This message is utilised for notifying the Babylon blockchain
that a staking transaction that was previously submitted through
the EOI process is now on Bitcoin and has received sufficient
confirmations to become active.

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

### Stake creation/registraton
#### Registering a phase-1 stake

To register your phase-1 stake to phase-2, you need to submit a `MsgCreateBTCDelegation` 
with a valid inclusion proof. This requires your staking transaction to already 
be confirmed on the Bitcoin network.

1. Create `MsgCreateBTCDelegation` with the inclusion proof filled as defined 
above and submit `MsgCreateBTCDelegation` to the Babylon network's btcstaking 
module. If all verifications pass, your delegation becomes active on the Babylon 
chain immediately since you've already proven your BTC is locked on Bitcoin.

2. You can check the activation status by querying the babylon node like this.

```bash
babyloncli query btcstaking delegation-status <staking-tx-hash>
```

3. Note that certain eligibility criteria apply including:
- Valid timelock periods (maximum 65535 blocks)
- Properly formatted transactions with correct signatures
- Valid Proof of Possession demonstrating control of your BTC private key.

#### Creating new stakes

Create and submit MsgCreateBTCDelegation with an empty inclusion proof to the Babylon network. The message should include:

- Your unsigned staking transaction
- Pre-signed slashing transaction
- Unsigned unbonding transaction
- Pre-signed unbonding slashing transaction
- Proof of Possession (PoP)
- Other required fields (staking amount, timelock periods, etc.)

2. Wait for covenant signatures to be collected. The covenant committee will verify your transactions and add their required signatures. This ensures your delegation will be accepted once you commit your BTC.

3. Once you have covenant approval, you can confidently submit your staking transaction to the Bitcoin network. This ensures your BTC won't be locked if the covenant rejects your delegation.

4. After your Bitcoin transaction is confirmed, the inclusion proof needs to be submitted via `MsgAddBTCDelegationInclusionProof`. While our Vigilante service will automatically handle this, you can also submit it yourself:
<!-- 
        <!-- 1. Create `MsgCreateBTCDelegation` with an empty inclusion proof
        2.  You can wait for covenant signatures to be collected
        3. Once they do and you feel confident, you can submit the staking tx to btc
        4. You can wait for inclusion, our vigilante will send the inclusion proof through `MsgInsertInclusionProof` . If you want however, you can also do it yourself.
        5. You can monitor for your stake status like this.. -->

#### On-demand unbonding
    1. You can retrieve covenant signatures from babylon chain and construct unbonding which is sent to btc
    2. Babylon will monitor etc..
## Withdrawing
    1. You can withdraw any time blah blah
    2. You can withdraw from slashing blah blah

