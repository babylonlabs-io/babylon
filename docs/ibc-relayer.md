# IBC relaying guide

Babylon uses [IBC](https://ibcprotocol.dev/)
(Inter-Blockchain Communication protocol) to enable cross-chain
communication with other Cosmos SDK chains. This guide focuses on the specific configurations needed
when relaying with Babylon, particularly around its unique unbonding period
mechanism.

## Important Note on Babylon's Unbonding Period

Babylon has a unique unbonding mechanism that differs from standard Cosmos SDK chains. The Babylon Genesis chain introduces secure, fast unbonding through Bitcoin timestamping:

1. **Epoching System**:
   - All staking operations and voting power adjustments are processed at the final block of each epoch
   - The final block of each epoch is checkpointed onto the Bitcoin blockchain
   - Each epoch spans 360 blocks (defined by `epoch_interval` parameter)
   - With 10s block times, each epoch duration is 1 hour

2. **Finalization Process**:
   - After an epoch is timestamped on a Bitcoin block, it becomes finalized once the block is 300-deep
   - This is defined by the `checkpoint_finalization_timeout` parameter
   - Any unbonding requests from that checkpointed epoch are then matured
   - Given Bitcoin's average block time of ~10 minutes, the average unbonding time is about 50 hours

3. **IBC Light Client Configuration**:
   - IBC light clients for Babylon Genesis on other chains should have a lower trusting period (~33 hours)
   - This is about 2/3 of the unbonding period, following standard IBC security practices
   - This configuration only affects light clients of Babylon Genesis on other chains
   - The trusting period of other chains' light clients on Babylon Genesis remains unaffected

4. **Module Configuration**:
   - The `x/staking` module is disabled and wrapped with `x/epoching` module
   - The standard `x/staking` module's unbonding time parameter remains at the default 21 days
   - **This 21-day value should be ignored** when configuring the relayer's trusting period

Due to these unique characteristics, special attention is required when configuring the relayer's trusting period and client refresh rate.

## Prerequisites

Before beginning, ensure you have:
1. Rust installed and configured
2. Hermes installed (refer to [Hermes Quick Start](https://hermes.informal.systems/quick-start/) for installation steps)
3. Access to RPC and gRPC endpoints for both chains
4. Wallets funded with native tokens for both chains

## Relayer Configuration

When setting up a relayer for Babylon, pay special attention to these parameters:

1. **Trusting Period**: Should be set to approximately 2/3 of Babylon's unbonding period
   - Babylon's unbonding period is ~50 hours (based on ~300 BTC blocks)
   - Therefore, the trusting period should be set to ~33 hours

2. **Client Refresh Rate**: A higher refresh rate is recommended (1/5 of trusting period, i.e., ~6.6 hours)
   - Make sure `refresh = true` is set in the configuration

**Important**: Do not use the default 21-day unbonding period that Hermes might fetch from the `x/staking` module query. Always set the trusting period based on Babylon's actual unbonding period of ~50 hours.

For complete setup instructions, including wallet configuration, connection setup, and channel creation, refer to:
- [Celestia's IBC Relayer Guide](https://docs.celestia.org/guides/ibc-relayer/)
- [Osmosis's Relayer Guide](https://docs.osmosis.zone/guides/relaying/relayer-guide)

## Monitoring and Maintenance

Regular monitoring of your IBC clients is crucial. The trusting period of 33 hours means you need to ensure your clients are refreshed before they expire. The client refresh rate of 6 hours ensures this happens well before the trusting period expires.

### IBC Metrics Monitoring

For effective monitoring, pay attention to these key metrics:

1. **Client Updates**: Monitor `client_updates_submitted_total` metric
   - This metric should consistently increase as more packets are relayed
   - A stagnant or decreasing value might indicate issues with client updates
   - For detailed information about this metric, refer to [Hermes metrics documentation](https://hermes.informal.systems/documentation/telemetry/operators.html#what-is-the-overall-ibc-status-of-each-network)

2. **Relayer Redundancy**: Monitor the level of redundancy in relayer operators
   - A large collection of unaffected packets can signal:
     - Network-wide IBC health issues
     - Relayer-specific problems (wallet issues, RPC problems)
   - Regular monitoring helps identify potential issues before they become critical

For advanced monitoring and insights, you can use tools like [Informal Systems' IBC Insights](https://insights.informal.systems/noble/osmosis) to track network health and relayer performance.

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