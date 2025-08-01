# Zone Concierge IBC Channel

- [Overview](#overview)
- [IBC Packets](#ibc-packets)
- [IBC Channel Information](#ibc-channel-information)
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

**BSN Registration Requirement:** A Cosmos BSN must be registered in Babylon
Genesis' consumer registry before establishing an associated Zone Concierge IBC
channel; otherwise, *creating an IBC channel associated with an unregistered BSN
will be rejected by the Zone Concierge module.*

That is, to integrate with Babylon Genesis as a Cosmos BSN, one needs to do the
following in order:

- Create an IBC light client or choose an IBC light client that is actively
  relayed for this Cosmos chain inside Babylon Genesis chain.
- Register the Cosmos chain as a BSN with the identifier being the ID of the IBC
  light client.
  - If Babylon Genesis chain specifies permissionless integration, the
    registration can be done through `babylond tx btcstkconsumer
    register-consumer <consumer-id> <name> <description>` where `<consumer-id>`
    has to be the IBC light client's ID.
  - If Babylon Genesis chain specifies permissioned integrtaion, the
    registration requires a governance proposal.
  - Please refer to [x/btcstkconsumer/README.md](../x/btcstkconsumer/README.md)
for more details about registering a Cosmos chain as a BSN.
- Establish the Zone Concierge IBC channel on top of the IBC light client
  associated with the registered BSN.

## Useful Documentations

- [./ibc-relayer.md](./ibc-relayer.md) - An IBC relayer guide for Babylon
- [Babylon ZoneConcierge Module](../x/zoneconcierge/README.md) - Zone Concierge
  module documentation
- [Cosmos BSN Contracts](https://github.com/babylonlabs-io/cosmos-bsn-contracts)
  - Cosmos BSN contracts implementation
- [BSN Integration
  Deployment](https://github.com/babylonlabs-io/babylon-bsn-integration-deployment)
  - Integration examples and artifacts
