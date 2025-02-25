## Introduction

##  Prerequisites

## Taproot outputs

## Spending Scripts

The following spending scripts define the conditions under which the
UTXO can be spent. These scripts are stored inside the Merkle tree
and are enforced when BTC is withdrawn.

### Slashing

Used when slashing a staker for misbehavior (e.g., double signing).

```shell
<StakerPk> OP_CHECKSIGVERIFY
<FinalityProviderPk> OP_CHECKSIGVERIFY
<CovenantPk1> OP_CHECKSIG <CovenantPk1> OP_CHECKSIGADD ... <CovenantPkN> OP_CHECKSIGADD
<CovenantThreshold> OP_NUMEQUAL
```

### Unbonding

Used when a staker wants to unbond their BTC before the timelock expires.

```shell
<StakerPk> OP_CHECKSIGVERIFY
<CovenantPk1> OP_CHECKSIG <CovenantPk1> OP_CHECKSIGADD ... <CovenantPkN> OP_CHECKSIGADD
<CovenantThreshold> OP_NUMEQUAL
```

### Timelock

Used when BTC is withdrawn after the staking period ends (no extra signatures needed).

```shell
<StakerPK> OP_CHECKSIGVERIFY  <TimelockBlocks> OP_CHECKSEQUENCEVERIFY
```

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

## Slashing

### Sending a TX

## Spending taproot outputs