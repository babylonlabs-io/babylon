# Staking Registration Eligibility

1. [Introduction](#1-introduction)
2. [Timeline of Events](#2-timeline-of-events)
    1. [Chain launch](#21-chain-launch)
    2. [Allow-list](#22-allow-list)
    3. [Staking protocol and finality activation](#23-staking-protocol-and-finality-activation)
3. [Retrieving details about the timeline](#3-retrieving-details-about-the-timeline)
4. [FAQs](#4-faqs)

## 1. Introduction

This document outlines the various stages of the Babylon chain launch
and details which actors are eligible to register at each stage.
The launch is structured into three key stages:

1. Chain Launch
2. Bitcoin Staking Finality activation
3. Uncapped Bitcoin Staking
<!-- todo improve the name of stage 3 -->

Each stage determines when and how different participants can register.
* **Stage 1: Chain Launch**: At this stage, only
  Finality providers and allow-listed stakes can register.
  The allow-list specifies a list of transaction hashes that
  are eligible for registration. These transaction hashes are
  typically associated with existing transactions (e.g., coming from phase-1).
  The main purpose of the allow-list is to enforce the initial slow onboarding
  of the Bitcoin stake to ensure a smooth launch before BTC staking
  becomes fully permissionless.
  Note that even though stake and finality providers can register,
  they do not have voting power. This comes at the next stage.
* **Stage 2: Bitcoin Staking Finality Activation** Finality providers gain
  voting rights, and BTC stakers begin receiving rewards.
* **Stage 3: Uncapped Bitcoin Staking**: All stakeholders can register and new
  stakes can be created. BTC Staking is uncapped.

Further details on this process can be found
in [Section 2.3: Staking Protocol & Finality Activation](#23-staking-protocol-and-finality-activation).

## 2. Terminology

### 2.1. Bitcoin Stake Registration

Bitcoin stake registration involves the submission of
Bitcoin stakes into the Babylon chain in order for the stake
to receive voting power and earn rewards. More details
on the different ways to register stake
can be found [here](../../../docs/register-bitcoin-stake.md).

### 2.2. Stakes Allow-List

The allow-list comprises a collection of transaction hashes that correspond to
specific stakes. This list is used to determine which stakes are eligible for
registration. During the active period of the allow-list, only post-staking
registrations are allowed, particularly during block production and finality
activation. Pre-staking registrations are not permitted until the allow-list
has expired.

The allow-list will expire at a predefined block height. Once it has expired,
all stake types, both pre-staking and
post-staking registrations become valid for staking.

### 2.3. Finality Voting Activation
..
* what does activation mean? Before activation who secures the chain?
* Why do we do this? I want to earn rewards now!
Given the limited **block production throughput**,
onboarding will occur over multiple blocks to prevent congestion.
Decentralization will be achieved gradually as more participants
join the network.

## 3. Timeline of Events

![Staking Timeline](./static/stakingtimeline.png)

### 3.1. Chain Launch

The chain launch procedure involves the Babylon Chain
starting to produce blocks that contain transactions.
At this point, the following actors can start onboarding
into the chain:
* **CometBFT Validators**: CometBFT validators can permissionlessly
  submit validator registration transactions and become eligible
  for producing Babylon blocks. More details on the CometBFT
  validator registration procedure can be found [here](../../../x/epoching).
* **Finality Providers**: Finality providers can permissionlessly
  register into the Babylon chain. Note that finality providers
  that have operated and received delegations in a Phase-1 Babylon
  network, should register using *exactly* the same EOTS key they
  used for the corresponding network
  (i.e., for Babylon chain mainnet use the same key as with the phase-1 mainnet).
  More details on how to register a finality provider or migrate the phase-1
  EOTS key into the Babylon chain can be found
  [here](https://github.com/babylonlabs-io/finality-provider).
* **BTC Stake Registration** Bitcoin stakes for which their hash
  is included in the allow-list and the finality provider to which
  they have been delegated to has registered
  can now register to the Babylon chain.
  More details on how to register your Bitcoin stakes
  [here](../../../docs/register-bitcoin-stake.md).

> **⚡ Important Note** Bitcoin stakes cannot register to the Babylon chain
> unless the finality provider they have been delegated to has registered.
> For phase-1 stakes, this means that the phase-1 finality provider
> should register first, before stake registration is attempted.

> **⚡ Important Note** While finality providers and Bitcoin Stakers can
> register at this point, the Bitcoin stake does not yet have voting power
> and is not eligible for receiving rewards. Voting power and rewards
> will start being granted once the finality protocol activates
> (see following sections).

### 3.2. Finality Voting Activation

* finality voting is activated. What can people do now?
* finality providers: will start voting for the finalization of blocks.
* rewards: btc staking rewards are going to be distirbuted to btc stakers based
* on their voting power.
* people can start withdrawing rewards.

There are 2 ways to create stake, through pre-staking registration and
post-staking registration.

* **Pre-staking registration**: The process where a staker registers their
    stake with Babylon **before** staking on Bitcoin, without providing a proof
    of inclusion.
* **Post-staking registration**: The process where a staker **first** stakes on
    Bitcoin and then registers their stake with Babylon.

To see more on this please refer to [Registering Bitcoin Stake](../../../docs/register-bitcoin-stake.md)

Babylon's BTC Staking protocol introduces an additional consensus round, known
as the **finality round**, which occurs after blocks are produced by CometBFT.
This round is conducted by **finality providers**, whose voting power is
derived from staked BTC delegated to them.
The [Finality module](https://github.com/babylonlabs-io/babylon/blob/main/x/finality)
leverages the voting power table
maintained in the BTC Staking module to determine block finalisation status,
detect equivocations among finality providers, and impose slashing penalties on
BTC delegations under culpable providers.

During the chain launch, finality providers and BTC stakes must transition
through an active Babylon blockchain in a structured sequence. Finality
providers must transition first, as the Babylon blockchain only accepts stake
registrations that delegate to known finality providers. Due to the network’s
limited block throughput, onboarding both finality providers and BTC stakes will
require multiple blocks to complete. Additionally, since CometBFT validators
have the ability to censor specific onboarding transactions, it is essential to
allow sufficient validators to produce blocks.

Additionally, the activation timing will be specified as a special condition on
`x/btcstaking` based on a pre-defined block height.

### 3.3. Allow-list Expiration

What can the actors of the system now do (i.e. what capabilities
are unlocked for them, once we reach this point)
* allow-list has now expired
* business as usual for fps
* stake registration is now open for every stake either existing
  (e.g. from phase-1) or new one

## 4. Retrieving details about the timeline
<!-- the below needs to be upated as cannot seem to find the query, i think it needs to be added:
https://github.com/babylonlabs-io/babylon/issues/321-->

To retrieve information about protocol activation, we can query the BTC Staking
module, where a predefined block height is set for activation.

The allow-list is hardcoded and accessible to all users; however, it is currently
only manageable by querying the genesis file. This is defined by the parameter:

```
`flagAllowedReporterAddresses = "allowed-reporter-addresses"`
```

To query the BTC Staking parameters, use:

`babylond query btcstaking params`

Within the output, look for the `BTCStakingActivatedHeight` field to find the
activation block height.

### 5. FAQs

<!-- TBD -->