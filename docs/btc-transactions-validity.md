## Introduction

##  Prerequisites

## Taproot outputs

## Spending Scripts

The following spending scripts define the conditions under which the UTXO can
be spent. These scripts are stored inside the Merkle treeand are enforced when
BTC is withdrawn.

### Slashing  Path

The Slashing Path is a predefined spending condition that penalises finality
providers and their delegators in the event of double signing or other
misbehavior. To enforce this path, Babylon commits a Slashing Script inside
the Taproot UTXO’s Merkle root (`m`), ensuring that BTC can only be
withdrawn under specific conditions.

The following script enforces the Slashing Path:

```shell
<StakerPk> OP_CHECKSIGVERIFY
<FinalityProviderPk> OP_CHECKSIGVERIFY
<CovenantPk1> OP_CHECKSIG <CovenantPk1> OP_CHECKSIGADD ... <CovenantPkN> OP_CHECKSIGADD
<CovenantThreshold> OP_NUMEQUAL
```

**Parameters:**

- `StakerPK` is the BTC staker's public key.
- `FinalityProviderPk` is the BTC public key of the finality provider to whom
    the staker delegates their stake
- `CovenantPk1..CovenantPkN` are the lexicographically sorted public keys
    of the current covenant committee members recognised by the Babylon chain.
- `CovenantThreshold` is a Babylon parameter denoting the minimum covenant
    committee member signatures are required.

The slashing process is designed to prevent malicious behaviour in
collaboration of the BTC staker, finality provider, and covenant
committee. It is used in following way:

- For stake to become active, staker must publish pre-signed slashing
    transaction.
- The covenant committee validates such transaction, and publish its own
    signatures.
- The only signature missing to send the slashing transaction is
    finality provider signature. If finality provider private key leaks due
    to infractions, anyone can sign slashing transaction and send
    slashing transaction to Bitcoin network.

#### Unbonding Path

The Unbonding Path is a predefined spending condition that allows a staker
to withdraw their BTC before the timelock expires. To enforce this path,
Babylon commits an Unbonding Script inside the Taproot UTXO’s Merkle root
(`m`), ensuring that BTC can only be withdrawn under specific conditions.

The following script enforces the Unbonding Path:

```shell
<StakerPk> OP_CHECKSIGVERIFY
<CovenantPk1> OP_CHECKSIG <CovenantPk1> OP_CHECKSIGADD ... <CovenantPkN> OP_CHECKSIGADD
<CovenantThreshold> OP_NUMEQUAL
```

**Parameters:**

- `StakerPK` is the BTC staker's public key
- `CovenantPk1..CovenantPkN` are the lexicographically sorted public
    keys of the current covenant committee recognized by the Babylon chain
- `CovenantThreshold` is a Babylon parameter specifying the number of how
    many covenant committee member signatures are required.

#### Timelock Path

The Timelock Path is a predefined spending condition that allows a BTC
staker to withdraw their BTC after the staking period ends, without
requiring any additional signatures. To enforce this path, Babylon
commits an Unbonding Script inside the Taproot UTXO’s Merkle root (`m`),
ensuring that BTC can only be withdrawn under specific conditions.

The following script enforces the Timelock Path:

```shell
<StakerPK> OP_CHECKSIGVERIFY  <TimelockBlocks> OP_CHECKSEQUENCEVERIFY
```

**Parameters:**

- `<StakerPK>` is the BTC staker's public key..
- `<TimelockBlocks>` is the lockup period denoted in Bitcoin blocks.
    The timelock comes into effect after the Bitcoin transaction has been
    included in a mined block. In essence, the script denotes that only the
    staker can unlock the funds after the timelock has passed. It must be
    lower than `65535`.

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

## Slashing

### Sending a TX

## Spending taproot outputs