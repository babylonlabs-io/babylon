# Introduction

## Prerequisites

## Taproot

### Taproot Public Key

The Taproot public key (`Q`) is constructed in a way that commits to a Merkle
root (`m`), which defines the allowed spending conditions. This ensures that
only predefined scripts (timelock, unbonding, slashing) can be used to spend
the UTXO.

To spend BTC, the spender must provide a valid script and a Merkle proof,
proving that the script was pre-committed inside `m`. Each spending path
corresponds to a specific spending script, which enforces the conditions
for unlocking the BTC.

The Taproot public key (`Q`) is derived using the following formula:

```shell
Q = P + hash(P||m)G
```

- **`Q`** : The **final Taproot public key** (used to lock BTC in a Taproot
    UTXO).
- **`P`** : The **internal public key** (a predefined key in Babylon staking).
- **`m`** : The **Merkle root**, which commits to all possible spending scripts
    (e.g., timelock, unbonding, slashing).
- **`G`** : The **elliptic curve generator point** (used in Bitcoin
    cryptographic operations).

For Staking transactions, the internal public key (`P`) is a predefined
fixed value. The exact internal public key can be seen below.

```shell
P = lift_x(0x50929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0)
```

Bitcoin verifies transactions by requiring the spender  to provide the correct
spending script (e.g., unbonding script), a Merkle proof linking the script to
the original Merkle root(`m`), and any necessary Schnorr signatures from the
staker, finality provider. Using the provided Merkle proof, Bitcoin recomputes
the Merkle root and checks if it matches `m`, the one originally committed
inside `Q`. If `m' == m`, the UTXO is validated and able to be spent.

This Taproot public key construction follows the principles outlined in
[BIP341](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki#constructing-and-spending-taproot-outputs),
which details how Taproot UTXOs commit to both a public key and an optional
script path.

The exact implementation for Babylon’s staking module can be found in the
[Babylon GitHub repository](https://github.com/babylonlabs-io/babylon/blob/main/btcstaking/types.go?plain=1#L27).

### Taproot Spending paths

As mentioned above, Babylon exclusively uses script path spending, meaning that
there are predefined rules as to how you can spend your BTC. These spending
paths are stored inside the Merkle root (`m`), which is committed in the
Taproot public key (`Q`).

- **Unbonding**: The unbonding path allows the staker to unlock their BTC
    before the timelock expires, but requires signatures from Babylon's Covenant
    Committee.
- **Slashing**: Used for penalising finality providers and their delegators in
    the case of double signing or misbehaviour.
- **Timelock**: Locks the staker's BTC for a fixed number of blocks, preventing
    withdrawal until the lock period expires.

As these paths are stored inside the Merkle root, a spender must provide a
valid script and Merkle proof to spend the UTXO via one of these paths

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
