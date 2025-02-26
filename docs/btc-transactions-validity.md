## Introduction

##  Prerequisites

## Taproot outputs

## Types of transactions

## Staking

### Sending a TX

### Unbonding Transaction

An Unbonding transaction is a special type of spending transaction that unlocks
BTC before the timelock expires. The paths are a predefined spending rules
enforced by a one of the spending scripts [here](#spending-scripts)

**Requirements**:

1. **Inputs**: The transaction input must reference the original staking UTXO,
which serves as the source of BTC to be unlocked. This is the Taproot Output
from the staking transaction.
    - Taproot Public Key (`Q`)
    - To unlock the UXTO, the spender must unlock something called the witness.
        This should contain proof required to spend a Taproot UXTO. This should
        contain:
            - Spending Script (Unbonding Script)
            - Merkle Proof (proving the script was pre-committed)
            -  Signatures (from the Staker and Covenant Committee)
            - If all conditions are met, Bitcoin verifies the transaction
            and allows the UXTO to be spent.
2. **Taproot Output** **(`unbonding_output`)**: Since Bitcoin does not
        support modifying an existing UTXO, the unbonding transaction creates a
        new Taproot UTXO (`unbonding_output`).
            - The new Merkle Root (`m'`) reflects this updated spending tree
            - This new UTXO commits to a new spending script tree, containing:
            - **Timelock Script**: Allows the staker to retrieve BTC after
                the unbonding period.
            - **Slashing Script**: Allows Babylon to slash the stake if needed.
            - **Taproot Public Key**: A new Taproot Public Key is created
                following the same [formula](#taproot-public-key).
            - **Merkle Root**: Inside `Q`, commits to spending rules
                (timelock, unbonding, slashing)
            - **Internal Public Key**: Used to derive `Q`
3. **Unbonding Output Value**:  The value in the new Taproot output
    (`unbonding_output`) must be:

```go
unbonding_output.value = staking_output.value - global_parameters.unbonding_fee
```

#### Constructing an Unbonding Transaction Using the Babylon Library

The Babylon staking library exposes the
[BuildUnbondingInfo](https://github.com/babylonlabs-io/babylon/blob/main/btcstaking/types.go?plain=1#416)
function which builds a valid unbonding output. This function generates an
`UnbondingInfo` struct.

```go
func BuildUnbondingInfo(
    stakerKey *btcec.PublicKey,
    fpKeys []*btcec.PublicKey,
    covenantKeys []*btcec.PublicKey,
    covenantQuorum uint32,
    unbondingTime uint16,
    unbondingAmount btcutil.Amount,
    net *chaincfg.Params,
) (*UnbondingInfo, error)
```

**Parameters**:

- `stakerKey`- must be the same key as the staker key in `staking_transaction`.
- `fpKeys` - must contain one key, which is the same finality provider key used
    in `staking_transaction`.
- `covenantKeys`- are the same covenant keys as used in `staking_transaction`.
- `covenantQuorum` - is the same quorum as used in `staking_transaction`.
- `unbondingTime` - is equal to `global_parameters.unbonding_time`.
- `unbondingAmount` - is equal to `staking_amount - global_parameters.unbonding_fee`.

## Spending from Unbonding, Timelock and Slashing Outputs

Once the unbonding transaction is created, the next step is spending from the
Taproot output. This requires constructing the witnesses, which includes as
mentioned above:
    - Correct full spending script (Timelock, Unbonding, or Slashing Script)
    - A Merkle Proof (showing the script was pre-committed)
    - The **required signatures** (from the Staker, Finality Provider, and
        Covenant Committee)

The steps to spend a taproot include:

- Retrieve the correct spending script (e.g., unbonding, slashing, timelock).
- Recompute the Merkle proof that links the script back to the original Taproot
    public key (`Q`).
- Construct the control block, which provides:
        - The leaf version
        - The internal public key (`P`)
        - A Merkle proof* proving the script was part of `m`
- Use this to construct the witness, which is required to unlock the BTC.

Each spending path corresponds to a Merkle tree leaf, which is stored inside
the `StakingInfo` struct:

```go
type StakingInfo struct {
StakingOutput *wire.TxOut
scriptHolder *taprootScriptHolder
timeLockPathLeafHash chainhash.Hash
unbondingPathLeafHash chainhash.Hash
slashingPathLeafHash chainhash.Hash }
```

## Constructing the Witness

To spend BTC from a Taproot UTXO (staking or unbonding output), the spender must
 provide a **witness**. The witness acts as proof that the spender is following
 one of the pre-committed spending rules.

To create transactions that spend from Taproot outputs, the spender must provide:

- The full script being spent (e.g., timelock, unbonding, or slashing) used to
    unlock the BTC.
- A control block (which contains: the leaf version, internal public key, and
    proof of inclusion of the script in the script tree)

**Purpose of the Control Block**:

The control block provides proof that the spending script is one of the
pre-committed scripts inside the Taproot Merkle tree

Since Bitcoin does not store the spending scripts, they must be reconstructed
to prove that the spender follows the original staking conditions.

### Re-creating Spending Script & Control Block

Instead of storing scripts, they can be **re-built on demand** (since they are
deterministically generated). The following function demonstrates how to
construct the **timelock script** and its control block.

```go
import (
 // Babylon btc staking library
 "github.com/babylonlabs-io/babylon/btcstaking"
)

func buildTimelockScriptAndControlBlock(
 stakerKey *btcec.PublicKey,
 finalityProviderKey *btcec.PublicKey,
 covenantKeys []*btcec.PublicKey,
 covenantQuorum uint32,
 stakingTime uint16,
 stakingAmount btcutil.Amount,
 netParams *chaincfg.Params,
) ([]byte, []byte, error) {

 stakingInfo, err := btcstaking.BuildStakingInfo(
  stakerKey,
  []*btcec.PublicKey{finalityProviderKey},
  covenantKeys,
  covenantQuorum,
  stakingTime,
  stakingAmount,
  netParams,
 )

 if err != nil {
  return nil, nil, err
 }

 si, err := stakingInfo.TimeLockPathSpendInfo()

 if err != nil {
  return nil, nil, err
 }

 scriptBytes := si.RevealedLeaf.Script

 controlBlock := si.ControlBlock

 controlBlockBytes, err := controlBlock.ToBytes()
 if err != nil {
  return nil, nil, err
 }

 return scriptBytes, controlBlockBytes, nil
}

```

### Using the Script & Control Block

The generated script and control block can be used in two ways:

1. Manually constructing the witness to sign and broadcast the transaction.
2. Embedding them into a PSBT (Partially Signed Bitcoin Transaction), which
    allows `bitcoind` to create the witness.

This approach ensures that Bitcoin will only accept spending transactions that
strictly follow the staking rules pre-committed in the original staking UTXO.

### Creating PSBT to get signature for given Taproot path from `Bitcoind`

Bitcoind’s [walletprocesspsbt](https://developer.bitcoin.org/reference/rpc/walletprocesspsbt.html)
automates witness generation, reducing the need for manual signature creation.
To use this `Bitcoind` endpoint to get signature/witness the wallet must
maintain one of the keys used in the script.

Example of creating psbt to sign unbonding transaction using unbonding script
from staking output:

``` go
import (
 "github.com/btcsuite/btcd/btcutil/psbt"
)

func BuildPsbtForSigningUnbondingTransaction(
 unbondingTx *wire.MsgTx,
 stakingOutput *wire.TxOut,
 stakerKey *btcec.PublicKey,
 spentLeaf *txscript.TapLeaf,
 controlBlockBytes []byte,
) (string, error) {
 psbtPacket, err := psbt.New(
  []*wire.OutPoint{&unbondingTx.TxIn[0].PreviousOutPoint},
  unbondingTx.TxOut,
  unbondingTx.Version,
  unbondingTx.LockTime,
  []uint32{unbondingTx.TxIn[0].Sequence},
 )

 if err != nil {
  return "", fmt.Errorf("failed to create PSBT packet with unbonding transaction: %w", err)
 }

 psbtPacket.Inputs[0].SighashType = txscript.SigHashDefault
 psbtPacket.Inputs[0].WitnessUtxo = stakingOutput
 psbtPacket.Inputs[0].Bip32Derivation = []*psbt.Bip32Derivation{
  {
   PubKey: stakerKey.SerializeCompressed(),
  },
 }

 psbtPacket.Inputs[0].TaprootLeafScript = []*psbt.TaprootTapLeafScript{
  {
   ControlBlock: controlBlockBytes,
   Script:       spentLeaf.Script,
   LeafVersion:  spentLeaf.LeafVersion,
  },
 }

 return psbtPacket.B64Encode()
}

```

Given that to spend through the unbonding script requires more than the
staker's signature, the `walletprocesspsbt` endpoint will produce a new
psbt with the staker signature attached.

In the case of a timelock path which requires only the staker's signature, 
`walletprocesspsbt` would produce the whole witness required to send the
transaction to the BTC network.
