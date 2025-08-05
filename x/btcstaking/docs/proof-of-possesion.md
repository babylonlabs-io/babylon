# Proof of Possession for Babylon BTC Staking
## Table of contents
1. [Overview](#1-overview)
2. [Domain Separation and Security](#2-domain-separation-and-security)
3. [Proof of Possession Implementation](#3-proof-of-possession-implementation)
   1. [Signature Methods](#31-signature-methods)
   2. [Payload Construction](#32-payload-construction)
   3. [Example Implementation](#33-example-implementation)

## 1. Overview

Proof of Possession (PoP) is a cryptographic signature that proves control of 
the private key corresponding to a claimed public key. In Babylon's BTC 
staking protocol, PoP signatures are required for BTC delegation creation 
(`MsgCreateBTCDelegation`) and finality provider registration 
(`MsgCreateFinalityProvider`) to prevent rogue key attacks and ensure only 
legitimate key owners participate in the protocol.

This specification defines the exact payload format and signature 
requirements for creating valid PoP signatures. Both BTC stakers and finality 
providers use the same PoP mechanism verified by the `x/btcstaking` module.

## 2. Domain Separation and Security

Valid proof of possession signatures must be made over a payload prepended
with the correct context string (also known as a domain separation tag).
This is critical for security to prevent replay attacks, ensuring that a
signature created for one purpose cannot be maliciously reused for another.

### The Context String Format

The context string provides uniqueness to prevent signature replay attacks 
and follows this format:

`{protocol_name}/{version}/{operation_tag}/{chain_id}/{address}`

**Parameters:**
- `protocol_name`: `btcstaking`
- `version`: `0`
- `operation_tag`: `staker_pop` or `fp_pop`
- `chain_id`: Target Babylon network chain ID
- `address`: `x/btcstaking` module bech32 address

## 3. Proof of Possession Implementation

### 3.1. Signature Methods

Three cryptographic signature methods are available:

-  `BIP-340`: Schnorr signature as defined by the BIP-340 standard
-  `BIP-322`: Generic transaction signing as defined by the BIP-322 standard
-  `ECDSA`: Standard ECDSA Bitcoin signature

### 3.2. Payload Construction

**Payload Construction Steps:**
1. Create the context string using the appropriate operation tag (`staker_pop` 
   for BTC stakers, `fp_pop` for finality providers)
2. Hash and encode the context string using SHA256 and convert to hex
3. Concatenate the hex-encoded hash with the user's Babylon address
4. Sign the final payload using your chosen signature method

The payload format is:

```
Payload = toHex(sha256(context_string)) || user_addr
```

Where:
- `context_string` follows the format described in 
  [Section 2](#2-domain-separation-and-security)
- `user_addr` is the Babylon bech32 address (staker or finality provider)
- `||` represents string concatenation

### 3.3. Example Implementation

**Example Parameters**:
- `ChainID`: `bbn-1`
- Module address (varies by context):
  - `x/btcstaking`: `bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah`
  - `x/finality`: `bbn1finality_module_address_here`
- User address: `bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

**Step-by-step payload creation**:

1. Create context string (using `staker_pop` for BTC stakers or `fp_pop` 
   for finality providers):
   ```
   "btcstaking/0/staker_pop/bbn-1/bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah"
   ```

2. Hash and hexify the context string:
   ```
   hex_hash = toHex(SHA256(context_string))
   = 392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511
   ```

3. Concatenate hex hash with user address:
   ```
   392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511bbn1gwwgppyx
   raq2nhjcgpalwfvwhk700vh2waemz8
   ```

4. Sign the final payload using your chosen signature method (`BIP-340`, 
   `BIP-322`, or `ECDSA`)
