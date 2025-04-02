# Staking Registration Eligibility

1. [Introduction](#1-introduction)
2. [Terminology](#2-terminology)
    1. [Bitcoin Stake Registration](#21-bitcoin-stake-registration)
    2. [Stakes Allow-List](#22-stakes-allow-list)
    3. [Finality Voting Activation](#23-finality-voting-activation)
3. [Timeline of Events](#3-timeline-of-events)
    1. [Chain Launch](#31-chain-launch)
    2. [Finality Voting Activation](#32-finality-voting-activation)
    3. [Allow-list Expiration](#33-allow-list-expiration)
4. [Retrieving details about the timeline](#4-retrieving-details-about-the-timeline)

## 1. Introduction

This document outlines the various stages of the Babylon Genesis chain launch
and details the points in which different Finality Providers
and BTC stakes are eligible for registration.
The launch is structured into three key stages:

* **Stage 1: Babylon Genesis Chain Launch**: At this stage, only
  Finality Providers and allow-listed stakes can register.
  The allow-list specifies a list of transaction hashes that
  are eligible for registration. These transaction hashes are
  typically associated with existing transactions (e.g., coming from Phase-1).
  The purpose of the allow-list is for Bitcoin stakes to be onboarded
  in two separate stages, with the bulk of the stakes coming in the second
  stage. This ensures a smooth launch focused on safety
  and incremental onboarding of stakes. After the allow-list expires
  at a pre-determined time, all stakes, including new ones, can register.
  Note that even though stakes and Finality Providers can register,
  they do not have voting power. This comes at the next stage.
* **Stage 2: Bitcoin Staking Finality Activation** Bitcoin Stake receives
  finality voting power leading to
  Finality Providers starting to submit finality votes
  and BTC stakers receiving staking rewards.
* **Stage 3: Uncapped Bitcoin Staking**: All stakeholders can register and new
  stakes can be created. BTC Staking is uncapped.

## 2. Terminology

### 2.1. Bitcoin Stake Registration

Bitcoin stake registration involves the submission of
Bitcoin stakes to the Babylon Genesis chain in order for the stake
to receive voting power and earn rewards.

There are 2 ways to create stakes, either through pre-staking registration or
post-staking registration.

* **Pre-staking registration**: The process in which a staker registers their
    stake on Babylon **before** staking on Bitcoin, without providing a proof
    of inclusion.
* **Post-staking registration**: The process in which a staker **first** stakes on
    Bitcoin and then registers the stake on Babylon.

To see more on this please refer to the
[Registering Bitcoin Stake](../../../docs/register-bitcoin-stake.md)
documentation.

### 2.2. Stakes Allow-List

The allow-list consists of a collection of transaction hashes corresponding
to Bitcoin staking transactions. It has a pre-determined expiration date and
is implemented as a mechanism to initially restrict the stakes that can register into the chain in order
to ensure the secure and gradual launch of the system (similar to the caps mechanism on Phase-1).
While the allow-list is active, **only post-staking registrations are allowed**
with Bitcoin staking transactions that have a hash included in the allow-list.
Pre-staking registrations are not permitted until the allow-list has expired.

> **⚡ Note**
> The allow-list will expire at a predefined block height. Once it has expired,
> all stake types, both pre-staking and post-staking registrations become
> valid for staking.

### 2.3. Finality Voting Activation

Another measure to ensure that the Bitcoin Staking protocol
is smoothly launched is the delayed
activation of the finality voting power of Bitcoin Stakes.
This delay is necessary because Babylon Chain enables the staking of
the Bitcoin asset, meaning that sufficient amount of time should be
given for the Bitcoin asset to be onboarded.

Finality voting activation refers to the point at which
Bitcoin Stakes receive finality voting power and when
Finality Providers, that such stakes are delegated to,
can begin casting votes to finalize blocks on the Babylon chain.
The time of the finality voting activation
is determined by a block height included in the Babylon chain
[x/btcstaking module](../README.md) parameters
and will be specified at launch time.

Once finality voting is activated, BTC stakers can start earning rewards
based on their voting power.

## 3. Timeline of Events

![Staking Timeline](./static/stakingtimeline.png)

### 3.1. Babylon Genesis Chain Launch

The Babylon Genesis launch procedure involves the chain
starting to produce blocks that contain transactions.
At this point, the following actors can start onboarding
onto the chain:

* **CometBFT Validators**: CometBFT validators can permissionlessly
  submit validator registration transactions and become eligible
  for producing Babylon blocks. More details on the CometBFT
  validator registration procedure can be found [here](../../../x/epoching).
* **Finality Providers**: Finality Providers can permissionlessly
  register to Babylon Genesis. Note that Finality Providers
  that have operated and received delegations in a Phase-1 Babylon
  network, should register using *exactly* the same EOTS key they
  used for the corresponding network
  (i.e., for Babylon Genesis mainnet use the same key as with the Phase-1 mainnet).
  More details on how to register a Finality Provider or migrate the Phase-1
  EOTS key to Babylon Genesis can be found
  [here](https://github.com/babylonlabs-io/finality-provider).
* **BTC Stake Registration** Bitcoin stakes for which their hash
  is included in the allow-list and the Finality Provider to which
  they have been delegated to has registered
  can now register to Babylon Genesis.
  More details on how to register your Bitcoin stakes
  [here](../../../docs/register-bitcoin-stake.md).

> **⚡ Important** Bitcoin stakes cannot register to Babylon Genesis
> unless the Finality Provider they have been delegated to has registered.
> For Phase-1 stakes, this means that the Phase-1 Finality Providers
> should register first, before stake registration is attempted.

> **⚠️ Warning** Phase-1 stakes should always follow the post-staking
> registration procedure. Following the pre-staking registration
> procedure for a Phase-1 stake will lead to this stake's inability
> to ever register on the Babylon Genesis chain.

> **⚡ Important** While Finality Providers and Bitcoin Stakers can
> register at this point, the Bitcoin stake does not yet have voting power
> and is not eligible for receiving rewards. Voting power and rewards
> will start being granted once the finality protocol activates
> (see following sections).

### 3.2. Finality Voting Activation

When finality voting is activated, Finality Providers can begin
participating in the voting process to finalize blocks based
on the voting power they have received from their Bitcoin Stake
delegations. Note that only the active set of Finality Providers
determined by a top-X ranking based on the Finality Providers'
stake can participate in voting. The number of active Finality Providers is determined
by a parameter of the [x/btcstaking](../README.md) module.

### 3.3. Allow-list Expiration

Once the allow-list has expired, stake registration becomes fully
open and uncapped, allowing both existing stakes (e.g., from Phase-1)
and new stakes to be registered. Finality Providers can continue their
operations as usual, maintaining their role in the network without
any changes.

## 4. Retrieving details about the timeline

To obtain information about the activation block height and the allow-list of
staking transactions, you can query the BTC Staking module or inspect the
relevant code.

**Retrieving the Activation Block Height**
The Finality Activation block height can be retrieved by the
[x/btcstaking](../README.md) parameters. You can retrieve those:

* through the CLI and an RPC node connection

```shell
babylond query btcstaking params --node <rpcnode>
```

* through an LCD/API node connection (you can find one
  for the Babylon Genesis public networks
  [here](https://github.com/babylonlabs-io/networks))
* by parsing through the upgrade handler responsible for specifying it
  (e.g., for testnet)

```shell
app/upgrades/v1/testnet/btcstaking_params.go
```

**Retrieving the Allow-list of Staking Transaction Hashes**
The transaction hashes in the allow-list are hardcoded in
the codebase for each different deployed
[network](https://github.com/babylonlabs-io/networks).
For example, the testnet allow-list transaction
hashes can be found here:

```shell
app/upgrades/v1/testnet/allowed_staking_tx_hashes.go
```
