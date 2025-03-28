# IBC relaying guide

Babylon uses [IBC](https://ibcprotocol.dev/)
(Inter-Blockchain Communication protocol) to enable cross-chain
communication with other Cosmos SDK chains. This guide focuses on the specific configurations needed
when relaying with Babylon, particularly around its unique unbonding period
mechanism.

## Important Note on Babylon's Unbonding Period

Babylon has a unique unbonding mechanism that differs from standard Cosmos SDK chains. The Babylon Genesis chain disables the standard `x/staking` module and wraps it with the `x/epoching` module, introducing secure, fast unbonding through Bitcoin timestamping.

> **Important**: The standard `x/staking` module's unbonding time parameter remains at the default 21 days, but **this value should be ignored** when configuring the relayer's trusting period.

1. **Epoching System**:
   - All staking operations and voting power adjustments are processed at the final block of each epoch
   - The final block of each epoch is checkpointed onto the Bitcoin blockchain
   - Each epoch spans 360 blocks (defined by `epoch_interval` parameter of [x/epoching module](https://github.com/babylonlabs-io/babylon/blob/main/x/epoching/README.md))
   - With 10s block times, each epoch duration is 1 hour

2. **Finalization Process**:
   - After an epoch is timestamped on a Bitcoin block, it becomes finalized once the block is 300-deep
   - This is defined by the `checkpoint_finalization_timeout` parameter of [x/btccheckpoint module](https://github.com/babylonlabs-io/babylon/blob/main/x/btccheckpoint/README.md)
   - Any unbonding requests from that checkpointed epoch are then matured
   - Given Bitcoin's average block time of ~10 minutes, the average unbonding time is about 50 hours

3. **IBC Light Client Configuration**:
   - IBC light clients for Babylon Genesis on other chains should have a lower trusting period (~33 hours)
   - This is about 2/3 of the unbonding period, following standard IBC security practices
   - This configuration only affects light clients of Babylon Genesis on other chains
   - The trusting period of other chains' light clients on Babylon Genesis remains unaffected

Due to these unique characteristics, special attention is required when configuring the relayer's trusting period and client refresh rate.

## Relayer Configuration

When setting up a relayer for Babylon, pay special attention to these parameters:

1. **Trusting Period**: Should be set to approximately 2/3 of Babylon's unbonding period
   - Babylon's unbonding period is ~50 hours (based on ~300 BTC blocks)
   - Therefore, the trusting period should be set to ~33 hours

2. **Client Refresh Rate**: A higher refresh rate is recommended (1/5 of trusting period, i.e., ~6.6 hours)

For example, in Hermes configuration:
```
[[chains]]
trusting_period = "33 hours"
refresh = true
client_refresh_rate = 1/5
```

> **Important**: Do not use the default 21-day unbonding period that might be fetched from the `x/staking` module query. Always set the trusting period based on Babylon's actual unbonding period of ~50 hours.

For complete setup instructions, including wallet configuration, connection setup, and channel creation, refer to:
- [Celestia's IBC Relayer Guide](https://docs.celestia.org/how-to-guides/ibc-relayer)
- [Osmosis's Relayer Guide](https://docs.osmosis.zone/osmosis-core/relaying/relayer-guide)

## Monitoring and Maintenance

Regular monitoring of your IBC clients is crucial. For example, if using Hermes, you can monitor the `client_updates_submitted_total` metric, which counts the number of client update messages submitted between chains. This metric should increase over time as your relayer submits updates to keep the IBC clients synchronized. For detailed information about this metric as well as other important metrics, refer to [Hermes metrics documentation](https://hermes.informal.systems/documentation/telemetry/operators.html#what-is-the-overall-ibc-status-of-each-network).

> **Note**: For advanced monitoring, see [Informal Systems' IBC Insights](https://insights.informal.systems/noble/osmosis).

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