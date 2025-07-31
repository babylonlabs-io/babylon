# Proof of Possession for Babylon BTC Staking
## Table of contents
1. [Introduction](#1-introduction)
2. [What is a Proof of Possession?](#2-what-is-a-proof-of-possession)
3. [Domain Separation and Security](#3-domain-separation-and-security)
4. [Proof of Possession for BTC Stakers](#4-proof-of-possession-for-btc-stakers)
   1. [When PoP is Required](#41-when-pop-is-required)
   2. [Signature Methods](#42-signature-methods)
   3. [Payload Construction](#43-payload-construction)
   4. [Example Implementation](#44-example-implementation)
5. [Proof of Possession for Finality Providers](#5-proof-of-possession-for-finality-providers)
   1. [When PoP is Required](#51-when-pop-is-required)
   2. [Payload Construction](#52-payload-construction)
   3. [Example Implementation](#53-example-implementation)

## 1. Introduction

This document provides comprehensive guidance on creating valid Proof of
Possession (PoP) signatures for all participants in the Babylon BTC staking
protocol. While the primary focus is on BTC stakers, finality providers are
also covered as they are essential participants in the staking ecosystem -
BTC stakers must delegate to registered finality providers to participate
in the protocol.

Both BTC stakers and finality providers use the same PoP mechanism and are
verified by the same `x/btcstaking` module, making this a complete
reference for all PoP implementations within the BTC staking protocol.

**Target Audience**: This document is intended as a technical reference for
developers implementing BTC staking and finality provider registration systems.
This includes wallet developers, staking platform builders, finality provider
operators, and protocol integrators who need to create valid PoP signatures
for interaction with the Babylon BTC staking protocol.

### 1.1. Scenarios for PoP Signatures

You'll need to create PoP signatures in these scenarios:

**For BTC Stakers:**
- When creating a BTC delegation (staking your Bitcoin to earn rewards)
- When expanding an existing stake with additional funds
- Required as part of the `MsgCreateBTCDelegation` transaction

**For Finality Provider Operators:**
- When registering as a finality provider on the Babylon network
- Required as part of the `MsgCreateFinalityProvider` transaction
- Must be done before you can receive delegations from BTC stakers

### 1.2. What This Guide Covers

This document covers:
1. What PoP signatures are and why they're required for security
2. How to construct the correct payload for signing
3. Step-by-step examples for both BTC stakers and finality providers

## 2. What is a Proof of Possession?

A Proof of Possession (PoP) is a cryptographic signature that proves one
controls the private key corresponding to a public key they claim to own. In
Babylon's BTC staking protocol, PoP signatures are required for both BTC
delegation creation and finality provider registration, ensuring that only
legitimate key owners can participate in the protocol.

**Why are PoP signatures necessary?**

Without PoP signatures, malicious actors could:
- Use someone else's public key to register as a finality provider
- Create delegations using public keys they don't control
- Launch "rogue key attacks" where they manipulate aggregated signatures

PoP signatures eliminate these risks by requiring each participant to demonstrate
cryptographic control over their claimed keys before they can participate in
the protocol.

## 3. Domain Separation and Security

Valid proof of possession signatures must be made over a payload prepended
with the correct context string (also known as a domain separation tag).
This is critical for security to prevent replay attacks, ensuring that a
signature created for one purpose cannot be maliciously reused for another.

### 3.1. The Context String Format

At the core of the signing process is the context string, which provides
uniqueness to the signature's intent. The context string follows this format:

`{protocol_name}/{version}/{operation_tag}/{chain_id}/{address}`

Here’s a breakdown of each component for the Proof of Possession:

-   `protocol_name`: Must be `btcstaking`
-   `version`: The current version is `0`
-   `operation_tag`: Varies depending on who is creating the PoP:
    -   `staker_pop` for BTC stakers
    -   `fp_pop` for finality providers
-   `chain_id`: The chain ID of the Babylon network (e.g., `bbn-1` for mainnet)
-   `address`: The bech32 address of the Cosmos SDK module that will verify the
    signature. For both staker and finality provider PoP, this is the address
    of the `x/btcstaking` module

> **⚡ Important**: The context string ensures that signatures cannot be
> replayed across different contexts, protocols, or chains. Always use the
> exact format and current parameters for your target network.

## 4. Proof of Possession for BTC Stakers

### 4.1. When PoP is Required

BTC stakers must provide their proof of possession when creating delegations
through `MsgCreateBTCDelegation` or `MsgBtcStakeExpand` messages:

```protobuf
// MsgCreateBTCDelegation is the message for creating a BTC delegation
message MsgCreateBTCDelegation {
  option (cosmos.msg.v1.signer) = "staker_addr";
  // staker_addr is the address to receive rewards from BTC delegation.
  string staker_addr = 1 [ (cosmos_proto.scalar) = "cosmos.AddressString" ];

  // pop is the proof of possession of btc_pk by the staker_addr.
  ProofOfPossessionBTC pop = 2;
  // Other fields omitted for brevity
  ...
}
```

### 4.2. Signature Methods

BTC stakers have three cryptographic signature methods available:

-  `BIP-340`: Schnorr signature as defined by the BIP-340 standard
-  `BIP-322`: Generic transaction signing as defined by the BIP-322 standard
-  `ECDSA`: Standard ECDSA Bitcoin signature

Each method requires that the payload being signed follows this format:

```
Payload = toHex(sha256(context_string)) || staker_addr
```

Where:
- `context_string` follows the format described in [Section 3.1](#31-the-context-string-format)
- `staker_addr` is the Babylon bech32 address that will receive staking rewards
- `||` represents string concatenation

### 4.3. Payload Construction

The payload construction process involves these steps:

1. Create the context string using the `staker_pop` operation tag
2. Hash and encode the context string** using SHA256 and convert to hex
3. Concatenate the hex-encoded hash with the staker's Babylon address
4. Sign the final payload using your chosen signature method

### 4.4. Example Implementation

**Example Parameters**:
- `ChainID`: `bbn-1`
- `x/btcstaking module address`: `bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah`
- `Staker address`: `bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

**Step-by-step payload creation**:

1. Create context string:
   ```
   "btcstaking/0/staker_pop/bbn-1/bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah"
   ```

2. Hash and hexify the context string:
   ```
   hex_hash = toHex(SHA256(context_string))
   = 392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511
   ```

3. Concatenate hex hash with staker address:
   ```
   392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8
   ```

4. Sign the final payload using your chosen signature method (`BIP-340`, `BIP-322`, or `ECDSA`)

> **⚡ Note**: The final concatenated string is what must be signed to create
> a valid proof of possession for your BTC staking transaction.

## 5. Proof of Possession for Finality Providers

### 5.1. When PoP is Required

Finality providers must provide their proof of possession when registering
through `MsgCreateFinalityProvider` messages:

```protobuf
// MsgCreateFinalityProvider is the message for creating a finality provider
message MsgCreateFinalityProvider {
  option (cosmos.msg.v1.signer) = "addr";
  // addr is the address to send reward to.
  string addr = 1 [ (cosmos_proto.scalar) = "cosmos.AddressString" ];
  
  // description defines the description terms for the finality provider.
  Description description = 2;
  
  // commission defines the commission rate of finality provider.
  string commission = 3 [
    (cosmos_proto.scalar) = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec"
  ];
  
  // btc_pk is the Bitcoin secp256k1 PK of this finality provider.
  // The PK follows encoding in BIP-340 spec.
  bytes btc_pk = 4 [ (gogoproto.customname) = "BtcPk" ];
  
  // pop is the proof of possession of btc_pk by the finality provider.
  ProofOfPossessionBTC pop = 5;
  
  // consumer_id is the consumer ID of the consumer that the finality
  // provider is operating for.
  // If consumer_id is empty, then the finality provider is operating
  // for Babylon.
  string consumer_id = 6;
}
```

### 5.2. Payload Construction

The payload construction process for finality providers follows the same steps
as BTC stakers, but uses the `fp_pop` operation tag:

1. Create the context string using the `fp_pop` operation tag
2. Hash and encode the context string using SHA256 and convert to hex
3. Concatenate the hex-encoded hash with the finality provider's Babylon address
4. Sign the final payload using your chosen signature method

The payload format is:

```
Payload = toHex(sha256(context_string)) || fp_addr
```

Where:
- `context_string` uses the `fp_pop` operation tag as described in [Section 3.1](#31-the-context-string-format)
- `fp_addr` is the Babylon bech32 address that will receive commission rewards
- `||` represents string concatenation

### 5.3. Example Implementation

**Example Parameters**:
- `ChainID`: `bbn-1`
- `x/btcstaking module address`: `bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah`
- `Finality provider address`: `bbn1s4ckh9405q0a3jhkwx9jkjrjjjjkwrx8s4v7k2`

**Step-by-step payload creation**:

1. Create context string:
   ```
   "btcstaking/0/fp_pop/bbn-1/bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah"
   ```

2. Hash and hexify the context string:
   ```
   hex_hash = toHex(SHA256(context_string))
   = 1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f809
   ```

3. Concatenate hex hash with finality provider address:
   ```
   1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f809bbn1s4ckh9405q0a3jhkwx9jkjrjjjjkwrx8s4v7k2
   ```

4. Sign the final payload using your chosen signature method (`BIP-340`, `BIP-322`, or `ECDSA`)

> **⚡ Note**: The final concatenated string is what must be signed to create
> a valid proof of possession for your finality provider registration.
