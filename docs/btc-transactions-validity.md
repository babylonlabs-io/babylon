## Introduction

## Prerequisites

## Taproot outputs

## Staking Transaction

A staking transaction is different to the other transactions. It creates a UTXO
and can only be spent by specific paths. The paths are a predefined spending
rules enforced by a one of the spending scripts [here] (#spending-scripts).

Requirements:

1. **Inputs**: References previous UTXOs, provides BTC
2. **Taproot Output (`staking_output`)**: The vault that locks BTC. Commits
    to a script tree composed of the three scripts (timelock, unbondng and slashing.)
    - **Taproot Public Key**: Created in the Taproot output, commits to `m`
        (Merkle root)
    - **Merkle Root**: Inside `Q`, commits to spending rules (timelock,
        unbonding, slashing)
    - **Internal Public Key**: Used to derive `Q`
3. **`OP_RETURN` Output**:  Contains Babylon metadata (tag, `staker_pk`,
    `finality_provider_pk`, `staking_time`, `version`, `global_parameters.tag`)

The following are **not yet** needed.
    - Merkle Proof
    - Spending Script
    - Schnorr signatures

### Data Required for Staking Output

The staking output (`staking_output`) is created based on both user input and
global parameters. The following data is required to construct this output:

|**Parameter**|**Source**|**Usage**|
|---|---|---|
|`staker_public_key`|User Input|Included in OP_RETURN and all scripts (timelock, unbonding, slashing).|
|`finality_provider_public_key`|User Input|Used in the slashing script (`<FinalityPk>`) and OP_RETURN output.|
|`staking_time`|User Input|Specifies the lockup duration (must be within `global_parameters.min_staking_time` and `max_staking_time`). Used in OP_RETURN and timelock script.|
|`covenant_committee_public_keys`|Fetched from `global_parameters`|List of covenant committee members used in unbonding and slashing scripts.|
|`covenant_committee_quorum`|Fetched from `global_parameters`|Defines the required threshold of covenant committee signatures for spending BTC.|
|`staking_amount`|User Input|Determines the BTC locked in the staking transaction (`staking_output.value`).|
|`btc_network`|User Input|Specifies the Bitcoin network (mainnet, testnet, regtest).|

These values are either provided by the user** when initiating a staking
transaction or retrieved from the global parameters. The final `staking_output`
commits these conditions within its Taproot structure.

### Building a Staking Transaction Using the Babylon Library

To simplify the creation of a staking transaction the Babylon staking library provides its own function.

#### Function Overview

The Babylon staking library exposes the
[BuildV0IdentifiableStakingOutputsAndTx](https://github.com/babylonlabs-io/babylon/blob/main/btcstaking/identifiable_staking.go?plain=1#L231)
function, which is used to construct the staking transaction. This function
generates the following:

- A Taproot UTXO (locks BTC using `Q`)
- An `OP_RETURN` output, storing the staking metadata

```go
func BuildV0IdentifiableStakingOutputsAndTx(
 tag []byte,
 stakerKey *btcec.PublicKey,
 fpKey *btcec.PublicKey,
 covenantKeys []*btcec.PublicKey,
 covenantQuorum uint32,
 stakingTime uint16,
 stakingAmount btcutil.Amount,
 net *chaincfg.Params,
) (*IdentifiableStakingInfo, *wire.MsgTx, error)
```

To construct the full staking transaction, the user must provide the following
parameters:

**Parameters**:

- `Tag` - 4 bytes, a tag which is used to identify the staking transaction
    among other transactions in the Bitcoin ledger. It is specified in the
    `global_parameters.Tag` field.
- `Version` - 1 byte, the current version of the OP_RETURN output.
- `StakerPublicKey` - 32 bytes, staker public key. The same key must be used
    in the scripts used to create the Taproot output in the staking transaction.
- `FinalityProviderPublicKey` - 32 bytes, finality provider public key. The
    same key must be used in the scripts used to create the Taproot output in
    the staking transaction.
- `StakingTime` - 2 bytes big-endian unsigned number, staking time. The same
    timelock time must be used in scripts used to create the Taproot output in
    the staking transaction.

**Where does this data go?**

For the full transaction, the provided parameters (listed above) are used to
construct two main outputs:

1. The Taproot UTXO (`staking_output`), which enforces the staking rules on
    Bitcoin.
2. The `OP_RETURN` output, which stores Babylon-specific metadata.

This function internally calls `BuildV0IdentifiableStakingOutputs()`, which:
    - Retrieves `global_parameters`.
    - Generates the OP_RETURN output using the parameters above.
    - Constructs the Taproot UTXO, embedding the staking conditions inside the
        Merkle root.

#### OP_RETURN Output

To store staking metadata on-chain, Babylon uses an OP_RETURN output. This
output is built using the `V0OpReturnData` struct:

```go
type V0OpReturnData struct {
 Tag                       []byte
 Version                   byte
 StakerPublicKey           *XonlyPubKey
 FinalityProviderPublicKey *XonlyPubKey
 StakingTime               uint16
}

```

<!-- should we still include serialisation format for OP_RETURN data? -->

### Suggested way of creaing and sending a staking transaction

Once the staking transaction has been constructed using
`BuildV0IdentifiableStakingOutputsAndTx` function, it must be funded,
signed and broadcasted to the Bitcoin network. The following steps outline
the suggested process using `bitcoind`:

1. Create a staker key in the `bitcoind` wallet.
2. Generate an unfunded and unsigned staking transaction using the Babylon function:

``` go
info, stakingTx, err := BuildV0IdentifiableStakingOutputsAndTx(tag, stakerKey, fpKey, covenantKeys, covenantQuorum, stakingTime, stakingAmount, net)

```

3. Serialise the transaction:

```shell
staking_transaction_hex=$(bitcoin-cli createrawtransaction [] '[{"txid": "previous_utxo", "vout": 0}]')
```

4. Fund the transaction (selects unspent outputs to cover the BTC amount
    and fees):

```shell
funded_staking_transaction_hex=$(bitcoin-cli fundrawtransaction "staking_transaction_hex")
```

5. Sign the transaction:

```shell
signed_staking_transaction_hex=$(bitcoin-cli signrawtransactionwithwallet "funded_staking_transaction_hex")

```

6. Broadcast the transaction to the Bitcoin network:

```shell
bitcoin-cli sendrawtransaction "signed_staking_transaction_hex"

```

This process ensures that the staking transaction is properly constructed,
signed, and included in the Bitcoin blockchain.
