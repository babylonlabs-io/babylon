# How to Create a Valid Proof of Possession

## Introduction

This document provides guidance for BTC stakers and finality providers on how
to construct a valid Proof of Possession (PoP) signatures for the Babylon
system.

The Proof of Possession signatures are verified on the Babylon chain at different
times depending on your role:

*   **For BTC Stakers**: PoP is verified when you submit a transaction to create
    a delegation. This ensures that you have control over the BTC private key
    corresponding to the staker public key you are using to stake.
*   **For Finality Providers**: PoP is verified when you register as a finality
    provider on the Babylon chain. This proves your ownership of the EOTS key
    that will be used for signing finality votes and other operational
    messages.

Valid proof of possession signature must be made over the payload pre-pended
with correct context string (otherwise known as domain separation tag.)

This is critical for security to prevent replay attacks,
ensuring that a signature created for one purpose cannot be maliciously reused
for another.

## The Context String (Domain separation tag) for PoP

At the core of the signing process is the context string. It provides
uniqueness to the signature's intent.

The context string has the following format:

`{protocol_name}/{version}/{operation_tag}/{chain_id}/{address}`

Here’s a breakdown of each component for the Proof of Possession:

-   `protocol_name`: Must be `btcstaking`.
-   `version`: The current version is `0`.
-   `operation_tag`: This varies depending on who is creating the PoP:
    -   `staker_pop` for BTC stakers.
    -   `fp_pop` for finality providers.
-   `chain_id`: The chain ID of the Babylon network (e.g., `bbn-1` for the main
    network).
-   `address`: The bech32 address of the Cosmos SDK module that will verify the
    signature. For both staker and finality provider PoP, this is the address
    of the `x/btcstaking` module.

## 1. Proof of Possession for BTC Stakers

BTC stakers must provide their proof of possession when creating delegation
through `MsgCreateBTCDelegation` or `MsgBtcStakeExpand` messages. e.g

```protobuf
// MsgCreateBTCDelegation is the message for creating a BTC delegation
message MsgCreateBTCDelegation {
  option (cosmos.msg.v1.signer) = "staker_addr";
  // staker_addr is the address to receive rewards from BTC delegation.
  string staker_addr = 1 [ (cosmos_proto.scalar) = "cosmos.AddressString" ];

  // pop is the proof of possession of btc_pk by the staker_addr.
  ProofOfPossessionBTC pop = 2;
  // Other fields left for brevity
  ...
}
```

They have  three signing methods at their disposal:
-  **BIP-340**: using Schnorr signature as defined by BIP-340 standard
-  **BIP-322**: using generic transaction signing as defined by BIP-322 standard
-  **ECDSA**: using standard ECDSA BTC signature

Every method expect that the payload being signed is:
```
Payload = toHex(sha256(context_string)) || staker_addr
```

### Example Creation of Payload for signing

Given:
- **chain-id** - `bbn-1`
- **`x/btcstaking` module address** - `bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah`
- **staker_addr** - `bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

Procedure to create payload to sign in BTC staker pop looks as follows:

1.  **Context String**: create appropriate context string
    `"btcstaking/0/staker_pop/bbn-1/bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah"`
2.  **Hash the context and hexify it**:
    `hex_hash = toHex(SHA256(Context String)) = 392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511`
3.  **Final Payload to sign**: Concatenate the hex-encoded hash and `staker-addr`:
    `392376b1ca863487087702a0f74e90d44cd1f339a5776687c591bf5402395511bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

This final string is what staker must sign on.

---

## 2. Proof of Possession for Finality Providers

Finality providers must provide their proof of possession when creating finality provider
through `MsgCreateFinalityProvider` message. e.g

```protobuf
message MsgCreateFinalityProvider {
  option (cosmos.msg.v1.signer) = "addr";
  // addr defines the address of the finality provider that will receive
  // the commissions to all the delegations.
  string addr = 1 [ (cosmos_proto.scalar) = "cosmos.AddressString" ];

  // pop is the proof of possession of btc_pk over the FP signer address.
  ProofOfPossessionBTC pop = 5;

  // Other fields left for brevity
  ...
```

The signature must be made with finality provider's EOTS key.
The payload being signed is:

```
Payload = toHex(sha256(context_string)) || addr
```

### Example Creation of Payload for signing

Given:
- **chain-id** - `bbn-1`
- **`x/btcstaking` module address** - `bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah`
- **addr** - `bbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

Procedure to create payload to sign in FP PoP looks as follows:

1.  **Context String**: create appropriate context string
    `"btcstaking/0/fp_pop/bbn-1/bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah"`
2.  **Hash the context and hexify it**:
    `hex_hash = toHex(SHA256(Context String)) = b46118edaf8d2e6c5d0728e4ad7380fff51c6ad07e02cba1862ccb77ed59b87f`
3.  **Concatenate context string with `addr`**:
    `b46118edaf8d2e6c5d0728e4ad7380fff51c6ad07e02cba1862ccb77ed59b87fbbn1gwwgppyxraq2nhjcgpalwfvwhk700vh2waemz8`

This final string is what staker must sign on.
