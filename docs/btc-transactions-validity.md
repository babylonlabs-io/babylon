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

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

## Slashing

### Sending a TX

## Spending taproot outputs
