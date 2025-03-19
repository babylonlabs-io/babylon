## Introduction

## Prerequisites

## Taproot outputs

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

### Slashing Transaction

The slashing transaction penalises a staker according to the slashing rate by
removing the original staked BTC. It consumes a staking output, reducing the
staker’s funds and sending a portion to a designated penalty address. The
remaining BTC is locked in a new Taproot UTXO with a timelock script.

Requirements:

1. **Inputs**: References a valid staking UTXO, from the original staking transaction.
    - The staking UTXO must be locked under a Taproot script, enforcing staking rules.
2. **Taproot Output (slashing_output)**: Locks remaining BTC in a new Taproot UTXO,
    committing to a timelock script.
3. **Slashed BTC Output**: Sends the slashed BTC to a penalty
    address (immediately spendable).
4. **Taproot Public Key (P)**: Created in the Taproot output, commits to `m` (Merkle root).
5. **Merkle Root (m)**: Inside `Q`, commits to spending rules (timelock).
6. **Internal Public Key (Q)**: Used to derive `P`, ensuring output security.

**Data Required for Slashing Output**
The slashing output is constructed based on both user input and global
parameters. The following data is required:

| **Parameter**                     | **Source**                         | **Usage** |
|------------------------------------|-----------------------------------|--------------------------------------------------------------------------------|
| **staker_public_key**              | User Input                    | Included in `OP_RETURN` and used in timelock script. |
| **finality_provider_public_key**   | User Input                    | Used in the slashing script(`<FinalityPk>`) and `OP_RETURN` output. |
| **slashing_timelock**              | Fetched from `global_parameters` | Defines how long remaining BTC is locked after slashing. |
| **covenant_committee_public_keys** | Fetched from `global_parameters` | List of covenant committee members who authorize slashing transactions. |
| **covenant_committee_quorum**      | Fetched from `global_parameters` | Defines the required threshold of signatures for spending BTC. |
| **staking_amount**                 | User Input                     | Determines the BTC originally locked in the staking transaction. |
| **btc_network**                    | User Input                     | Specifies the Bitcoin network |

---

#### Building a Slashing Transaction Using the Babylon Library

To simplify the creation of a slashing transaction, the Babylon staking library
provides a function.

##### Function Overview

The Babylon staking library exposes the `buildSlashingTxFromOutpoint` function,
which constructs the slashing transaction:

```go
func buildSlashingTxFromOutpoint(
 stakingOutput wire.OutPoint,
 stakingAmount, fee int64,
 slashingPkScript []byte,
 changeAddress btcutil.Address,
 slashingRate sdkmath.LegacyDec,
) (*wire.MsgTx, error)
```

This function generates:

- A Taproot UTXO (`slashing_output`) that locks the remaining BTC to a timelock.
- A payment output that transfers the slashed BTC to a penalty address.

To construct the full staking transaction, the user must provide the
following parameters:

**Parameters**:

- `stakingOutput`: The UTXO from the original staking transaction (being slashed).
- `stakingAmount`: The total BTC locked in the staking UTXO before slashing.
- `fee`: The transaction fee for processing the slashing transaction.
- `slashingPkScript`: The script that dictates where the slashed BTC will be sent (penalty address).
- `changeAddress`: The address where the remaining BTC will be locked under Taproot.
- `slashingRate`: The percentage of BTC to be slashed, expressed as a decimal.

**Where Does This Data Go?**

For the full slashing transaction, the parameters above construct two main outputs:

1. The Taproot UTXO (slashing_output) stores the remaining BTC after slashing
    and commits to the timelock scripts in its Merkle root.
2. The Slashed BTC Output sends the penalized BTC to a designated penalty address.
    This output is immediately spendable.

**Suggested Way of Creating and Sending a Slashing Transaction**

Once the slashing transaction is constructed using `buildSlashingTxFromOutpoint`,
it must be funded, signed, and broadcasted. Below is the suggested workflow using `bitcoind`:

1. Generate an Unfunded, Unsigned Slashing Transaction
Use the Babylon function to create a raw slashing transaction:

```go
slashingTx, err := buildSlashingTxFromOutpoint(
    stakingOutput, stakingAmount, fee,
    slashingPkScript, changeAddress, slashingRate
)
if err != nil {
    log.Fatalf("Error building slashing transaction: %v", err)
    }
```

2. Serialize the Transaction
Convert the raw transaction to a hex format:

```shell
slashing_transaction_hex=$(bitcoin-cli createrawtransaction [] '[{"txid": "staking_utxo", "vout": 0}]')
```

3. Fund the Transaction
Select unspent UTXOs to cover the BTC amount and transaction fees:

```bash
funded_slashing_transaction_hex=$(bitcoin-cli fundrawtransaction "slashing_transaction_hex")
```

4. Sign the Transaction
Sign the funded transaction using the Bitcoin wallet:

```shell
signed_slashing_transaction_hex=$(bitcoin-cli signrawtransactionwithwallet "funded_slashing_transaction_hex")
```

5. Broadcast the Transaction
Send the signed slashing transaction to the Bitcoin network:

```shell
bitcoin-cli sendrawtransaction "signed_slashing_transaction_hex"
```
