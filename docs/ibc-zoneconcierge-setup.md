# Zone Concierge IBC Channel

- [Overview](#overview)
- [IBC Packets](#ibc-packets)
- [IBC Channel Information](#ibc-channel-information)
- [Establishing and Maintaining Zone Concierge IBC Channels](#establishing-and-maintaining-zone-concierge-ibc-channels)
  - [Overview of steps](#overview-of-steps)
  - [Step 1: IBC Light Client Setup](#step-1-ibc-light-client-setup)
  - [Step 2: BSN Registration](#step-2-bsn-registration)
  - [Step 3: Establishing zone concierge IBC channel](#step-3-establishing-zone-concierge-ibc-channel)
- [Useful Documentations](#useful-documentations)

This page provides instructions for setting up a Zone Concierge IBC channel
between Babylon Genesis' [ZoneConcierge module](../x/zoneconcierge/) and [Cosmos
BSN contracts](https://github.com/babylonlabs-io/cosmos-bsn-contracts) deployed
on Cosmos Bitcoin Supercharged Networks (BSNs) for BTC staking integration.

## Overview

The Zone Concierge module is an IBC-enabled module. It serves as Babylon
Genesis' gateway for communicating with BSNs. It leverages the IBC protocol to
synchronise information and provide BTC staking security for BSNs. For detailed
technical information of the Zone Concierge module, refer to
[`x/zoneconcierge/README.md`](../x/zoneconcierge/README.md).

## IBC Packets

The Zone Concierge module involves the following IBC packets.

**Outbound Packets (Babylon Genesis → BSN):**

- `BTCHeaders` - BTC headers
- `BTCTimestamp` - BTC timestamps for BTC headers
- `BTCStakingIBCPacket` - BTC staking events related to BSNs

**Inbound Packets (BSN → Babylon Genesis):**

- `BSNSlashingIBCPacket` - Slashing evidences from BSNs
- `BSNBaseBTCHeaderIBCPacket` - Base BTC headers from BSNs

For detailed protocol buffer definitions, see
[`proto/babylon/zoneconcierge/v1/packet.proto`](../proto/babylon/zoneconcierge/v1/packet.proto).

## IBC Channel Information

The IBC communication uses the following configuration:

| Setting | Value |
|---------|-------|
| **Port at Babylon Genesis** | `zoneconcierge` |
| **Port at BSN** | `wasm.$BABYLON_CONTRACT_ADDRESS` |
| **Channel Ordering** | `ORDERED` |
| **Protocol Version** | `zoneconcierge-1` |

Here `$BABYLON_CONTRACT_ADDRESS` is the address of the [Babylon
contract](https://github.com/babylonlabs-io/cosmos-bsn-contracts/tree/main/contracts/babylon)
deployed on the Cosmos BSN.

## Establishing and Maintaining Zone Concierge IBC Channels

Creating and maintaining zone concierge IBC channels is slightly different from
other IBC channels. In particular, before establishing an associated Zone
Concierge IBC channel, a Cosmos BSN must be registered in Babylon Genesis'
consumer registry. Otherwise, *creating an IBC channel associated with an
unregistered BSN will be rejected by the Zone Concierge module.*

This section provides instructions for establishing and maintaining zone
concierge IBC channels.

### Overview of steps

To integrate with Babylon Genesis as a Cosmos BSN, one needs to do the following
in order:

1. Create an IBC light client or choose an IBC light client that is actively
   relayed for this Cosmos chain inside Babylon Genesis chain.
2. Register the Cosmos chain as a BSN with the identifier being the ID of the
   IBC light client inside Babylon Genesis chain.
3. Establish the Zone Concierge IBC channel on top of the IBC light client
   associated with the registered BSN.

Note that creating IBC light client and creating IBC channel are two separate
steps, between which we need to register the IBC light client inside Babylon as
a BSN. This is because Babylon only allows creating IBC zone concierge channel
with chains that are registered as BSNs, and the chains are uniquely identified
using their IBC light clients as per the [IBC security
model](https://ibcprotocol.dev/how-ibc-works).

### Step 1: IBC Light Client Setup

Create an IBC light client or choose an IBC light client that is actively
relayed for this Cosmos chain inside Babylon Genesis chain.

You can either:

- Create a new IBC light client specifically for your BSN. Please refer to
  [Hermes relayer doc](https://hermes.informal.systems/) or [Go relayer
  doc](https://github.com/cosmos/relayer/blob/main/README.md) for creating IBC
  light clients.
- Use an existing IBC light client that is already actively relayed and
  maintained for your Cosmos chain

### Step 2: BSN Registration

Register the Cosmos chain as a BSN with the identifier being the ID of the IBC
light client. Registering BSN can be permissionless or permissioned
(governance-gated), depending on Babylon Genesis chain's parameter
`permissioned_integration` in `x/btcstkconsumer` module.

**If `permissioned_integration == false`:** Registering BSN is permissionless.
One can execute

```bash
$ babylond tx btcstkconsumer register-consumer $consumer-id $name $description $commission
```

where

- `$consumer-id` is the IBC light client ID inside Babylon Genesis
- `$name` is the name of the BSN
- `$description` is the description of the BSN
- `$commission` is the the commission rate (between 0 and 1) that Babylon
  charges for this BSN.

**If `permissioned_integration == true`:** Registering BSN is permissioned,
i.e., gated by governance. One can then submit a governance proposal for
registering the BSN. The governance proposal includes a `MsgRegisterConsumer`
[message](https://github.com/babylonlabs-io/babylon/blob/397d8dfae74c33ca089d1a31296e6dd40aa3e28c/proto/babylon/btcstkconsumer/v1/tx.proto#L43-L69)
as follows.
<!-- TODO: have a link to a real example of such gov prop -->

```json
{
  "messages": [
    {
      "@type": "/babylon.btcstkconsumer.v1.MsgRegisterConsumer",
      "signer": $gov_authority,
      "consumer_id": $consumer-id,
      "consumer_name": $name,
      "consumer_description": $description,
      "babylon_rewards_commission": $commission,
    }
  ],
  "metadata": "Register consumer BSN",
  "title": "Register consumer BSN",
  "summary": "Register consumer BSN for Babylon system",
  "deposit": $deposit_amount,
}
```

### Step 3: Establishing zone concierge IBC channel

After registering the BSN, one can establish the Zone Concierge IBC channel on
top of the consumer ID. Please refer to [Hermes relayer
doc](https://hermes.informal.systems/) or [Go relayer
doc](https://github.com/cosmos/relayer/blob/main/README.md) for creating an IBC
channel, and refer to [IBC Channel Information](#ibc-channel-information)
section for the IBC channel configurations.

## Useful Documentations

- [./ibc-relayer.md](./ibc-relayer.md) - An IBC relayer guide for Babylon
- [Babylon ZoneConcierge Module](../x/zoneconcierge/README.md) - Zone Concierge
  module documentation
- [Cosmos BSN Contracts](https://github.com/babylonlabs-io/cosmos-bsn-contracts)
  - Cosmos BSN contracts implementation
- [BSN Integration
  Deployment](https://github.com/babylonlabs-io/babylon-bsn-integration-deployment)
  - Integration examples and artifacts
