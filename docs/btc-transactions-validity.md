## Introduction

##  Prerequisites

## Taproot outputs

## Actors involved

There are three actors involved in the staking process, each identified based
on the signatures required for transaction validation.

1. **Bitcoin Staker**: Identified in the staking scripts by `<StakerPk>`,
    serving as the entity that locks BTC into the staking system
2. **Finality Provider**: Identified in the staking scripts by
    `<FinalityProviderPk>` and votes in the finality round to provide security
    assurance to the PoS chain.
3. Covenant Committee: This is of a select group of responsible for
    overseeing specific -related actions. They are represented by
    `CovenantPk1..CovenantPkN` which are the lexicographically
    sorted public keys of the covenant committee recognized by
    the Babylon chain.

## Types of transactions

## Staking

### Sending a TX

## Unbonding

### Sending a TX

## Spending taproot outputs

## Slashing

### Sending a TX

## Spending taproot outputs