# IBC relaying guide

Babylon uses [IBC](https://ibcprotocol.dev/)
(Inter-Blockchain Communication protocol) to enable cross-chain
transfer of tokens. This guide focuses on the specific configurations needed
when relaying with Babylon, particularly around its unique unbonding period
mechanism.

## Important Note on Babylon's Unbonding Period

Babylon has a unique unbonding mechanism that differs from standard Cosmos SDK chains:

1. Babylon uses Bitcoin timestamp-assisted unbonding (~300 BTC blocks, approximately 50 hours)
2. The standard `x/staking` module is disabled and wrapped with `x/epoching` module
3. Hermes by default queries the `x/staking` module for unbonding period, which will return 21 days on Babylon - **this value should be ignored**

Due to these unique characteristics, special attention is required when configuring the relayer's trusting period and client refresh rate.

## Prerequisites

Before beginning, ensure you have:
1. Rust installed and configured
2. Hermes installed (refer to [Hermes Quick Start](https://hermes.informal.systems/quick-start/) for installation steps)
3. Access to RPC and gRPC endpoints for both chains
4. Wallets funded with native tokens for both chains

## Configuration Requirements

When setting up a relayer for Babylon, pay special attention to these parameters:

1. **Trusting Period**: Should be set to approximately 2/3 of Babylon's unbonding period
   - Babylon's unbonding period is ~50 hours (based on ~300 BTC blocks)
   - Therefore, the trusting period should be set to ~33 hours

2. **Client Refresh Rate**: Should be set to 1/5 of the trusting period
   - With a 33-hour trusting period, the client refresh rate should be ~6 hours
   - This ensures clients are refreshed well before they expire

**Important**: Do not use the default 21-day unbonding period that Hermes might fetch from the `x/staking` module query. Always set the trusting period based on Babylon's actual unbonding period of ~50 hours.

For complete setup instructions, including wallet configuration, connection setup, and channel creation, refer to:
- [Celestia's IBC Relayer Guide](https://docs.celestia.org/guides/ibc-relayer/)
- [Osmosis's Relayer Guide](https://docs.osmosis.zone/guides/relaying/relayer-guide)

## Monitoring and Maintenance

Regular monitoring of your IBC clients is crucial. The trusting period of 33 hours means you need to ensure your clients are refreshed before they expire. The client refresh rate of 6 hours ensures this happens well before the trusting period expires.

For monitoring commands and troubleshooting, refer to the [Hermes documentation](https://hermes.informal.systems/documentation/commands/index.html).

## Handling Expired/Frozen IBC Clients

If an IBC client expires or becomes frozen, you'll need to submit a governance proposal to recover the client. This proposal needs to be submitted on the chain that maintains the light client of the counterparty chain.

For example, if you're relaying between Babylon and another chain:
- If Babylon's light client of the other chain expires, submit the proposal on Babylon
- If the other chain's light client of Babylon expires, submit the proposal on the other chain

For detailed steps on how to submit an IBC client recovery proposal, refer to
the [IBC Governance Proposals Guide](https://ibc.cosmos.network/main/ibc/proposals.html#steps).
For more information about submitting governance proposals on Babylon, including
parameters and requirements, see
the [Babylon Governance Guide](https://docs.babylonlabs.io/guides/governance/). 