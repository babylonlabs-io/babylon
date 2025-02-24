## Introduction

## Prerequisites

- [BIP341](https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki)-
a document specifying how to spend Taproot outputs

## How BTC Staking Works in Babylon

Babylon enables BTC staking by leveraging Bitcoin's Taproot functionality.
When BTC is staked, a staking transaction (defined by Babylon) outlines
how the staked BTC can be spent later. Once this transaction is confirmed, a
UTXO (Unspent Transaction Output) is created, locking the BTC under predefined
conditions.

A UTXO can be thought of as a vault, to unlock it, you must solve a specific
cryptographic puzzle. This vault is locked using a Taproot Public Key, which
commits the staking conditions using a Merkle root. However, Bitcoin
does not store the staking rules explicitly. Instead, it only stores `Q`,
which indirectly enforces them. This is explained further in the next section.

In Bitcoin, there are two types of UTXO spending methods:

1. **Key Path Spending** – Requires only a private key signature
    (not used in Babylon).
2. **Script Path Spending** – Requires execution of a Bitcoin Script
    (used in Babylon).

Babylon exclusively uses script spending to enforce specific staking conditions,
ensuring that BTC can only be unlocked under predefined rules. It is important
to note that the staking transaction is not a spending transaction; rather, it
creates a Taproot UTXO that locks the BTC. In contrast, unbonding, slashing,
and timelock transactions are spending transactions that unlock BTC by following
one of the pre-committed script paths. Once BTC is staked, it remains locked
until it is spent through one of these three allowed paths.

For example, to process an unbonding transaction, the BTC staker must:

- Provide **three valid signatures** (e.g., from the staker and the
    Babylon covenant committee).
- Submit a **valid Merkle proof** proving that the spending script was
    pre-committed in the staking transaction.

If all conditions are met, Bitcoin allows the UTXO to be spent,
unlocking the staked BTC.

If the conditions are not satisfied, Bitcoin rejects the transaction.

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

## Slashing

### Sending a TX

## Spending taproot outputs