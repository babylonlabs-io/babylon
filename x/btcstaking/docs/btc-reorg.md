# Large BTC Reorgs Recovery Procedure

Babylon relies on a number of confirmation blocks on Bitcoin to accept BTC
staking transactions. This number is defined inside the `x/btccheckpoint`
Parameter as `BtcConfirmationDepth` and as `k` in the research papers
which we consider to be an irreversible block.

If for some unexpected reason there is a reorg in the Bitcoin blockchain
larger than the `BtcConfirmationDepth`, the chain should halt and revoke all
the BTC delegations that were recorded until the first reorg block height.

By example, if the Bitcoin chain is in block height 150 and Babylon chain has a
`BtcConfirmationDepth` of 10 and it is submitted a valid Bitcoin reorg of 15 blocks
that rollbacks Bitcoin to block height 136, every BTC delegation that was included
in Bitcoin block 136 or higher should be deleted as the inclusion proof will
be different. BTC delegations that are included in the Bitcoin block height 135 or lower
will not be affected as the Bitcoin rollbacks only happens if there is a better
chain presented. A better chain means a chain that has more blocks, so any BTC
delegation included in block 135 or prior would still be considered valid.

This recovery procedure only cares about the BTC transactions that were executed
in blocks which have been rollbacked.

## Chain Halt

If the chain is halted due to a Bitcoin chain reorg larger than `BtcConfirmationDepth`,
there are a few steps to be followed to gather the data to be modified by the
emergency upgrade handler:

### 1. Collect Corresponding Babylon Height

The correlation of Bitcoin and Babylon block heights can be found in the `x/btcstaking`
state in the [BlockHeightChains](https://github.com/babylonlabs-io/babylon/blob/fcc6fdc009e414da440426e6b81920ceef981de3/x/btcstaking/types/genesis.pb.go#L36),
currently there is no query for it, but it can be retrieved by exporting the
genesis state.

### 2. Collect Bitcoin Related Messages that were Rollbacked

After having collected the Babylon block height corresponding to the Bitcoin block
height which the rollback happened, iterate over every transaction until
the halt block, then gather all the messages which were approved and are of one of
the types: `MsgCreateBTCDelegation`, `MsgAddBTCDelegationInclusionProof`,
`MsgBTCUndelegate`, `MsgInsertHeaders` and `MsgInsertBTCSpvProof`.

### 3. Analyze Each Message

Each Bitcoin related message that was rollbacked might need to have a different treatment.

#### 3.1 `MsgCreateBTCDelegation`

The message which creates a new Bitcoin delegation, if it doesn't have inclusion proof
can stay as it was not active, and neither had the inclusion
proof been sent.

If there was an inclusion proof, verify if the proof is from one of the Bitcoin block
heights which was rollbacked. If it is from a Bitcoin block that was not reorg,
but it was only later included in the Babylon chain, this is good and there is
nothing to do. Now if it was rollbacked, this BTC staking transaction needs to
be removed from the following modules states `x/btcstaking`, `x/finality`,
`x/incentives`.

- `x/btcstaking`
  - Remove from [`BTCDelegatorKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L22)
  the BTC staking transaction hash.
  - Remove from [`BTCDelegationKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L23)
  the BTC staking transaction hash.
  - Remove the [`PowerDistUpdateKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L27)
  if a `EventPowerDistUpdate_BtcDelStateUpdate` was emitted for that
  BTC staking transaction hash.
- `x/finality`
  - Update the voting power amount in [`VotingPowerKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L50)
  by subtracting the satoshi amounts of the Finality Provider that the BTC
  delegation delegated to if that BTC delegation was active.
  - Subtract the `TotalVotingPower` and the respective `FinalityProviderDistInfo`
  based on which Finality Provider was delegated to in
  [`VotingPowerDistCacheKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L51)
  if that BTC delegation was active.
- `x/incentives` If that BTC delegation was ever active, there is a need to
subtract the voting power from the rewards tracker. Also it only modifies
the values from now on. If rewards were accrued from a BTC staking transaction
that was rollbacked, the past is behind us, and it should be left with the funds
that were accrued, losing a few coins in the rewards.
  - For the incentives module state there is a single function which should be
  called [`BtcDelegationUnbonded`](https://github.com/babylonlabs-io/babylon/blob/c8c44be12eb826b41f6f2cd3eae4452268398cdf/x/incentive/keeper/reward_tracker.go#L47)
  which receives as parameter the Finality Provider and BTC delegator Babylon
  address and the amount of satoshis from the rollbacked BTC delegation.
  This will already update all the keys in the state accordingly.

> Note transactions that were included during blocks 150 ~ 141 (150 - `BtcConfirmationDepth`)
in a 15 Bitcoin blocks rollback are invalid and should be removed but were
also not considered deep enough to be active. Dropping those proofs and
the event updates to voting power table is enough to handle during the
recovery procedure.

#### 3.2 `MsgAddBTCDelegationInclusionProof`

Check if the proof is one from the BTC block heights which was rollbacked,
if it is not, there is no need to do anything. If it was rollbacked
the same steps defined in [3.1](#31-msgcreatebtcdelegation) need to be taken
to remove the data from the 3 modules states.

#### 3.3 `MsgBTCUndelegate`

Check if the inclusion proof had a reorg in the BTC blocks, if it is not
rollbacked, there is nothing to do. If it was rollbacked, remove the existence of this BTC undelegation and the affecting 3 modules states with similar steps taken at [3.1](#31-msgcreatebtcdelegation).

- `x/btcstaking`
  - Clean out the field for `BtcUndelegation` in `BTCDelegation` for the key
  [`BTCDelegationKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L23)
  in which corresponds to the BTC staking transaction hash.
  - Remove the [`PowerDistUpdateKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L27)
  if a `EventPowerDistUpdate_BtcDelStateUpdate` was emitted for that
  BTC staking transaction hash with Undelegate.
- `x/finality`
  - Update the voting power amount in [`VotingPowerKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L50)
  by adding the satoshi amounts of the Finality Provider that the BTC
  delegation delegated to.
  - Add the `TotalVotingPower` and the respective `FinalityProviderDistInfo`
  based on which Finality Provider was delegated to in
  [`VotingPowerDistCacheKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L51).
- `x/incentives` It is needed to add the voting power from the rewards tracker.
  - For the incentives module state there is a single function which should be
  called [`BtcDelegationActivated`](https://github.com/babylonlabs-io/babylon/blob/c8c44be12eb826b41f6f2cd3eae4452268398cdf/x/incentive/keeper/reward_tracker.go#L34)
  which receives as parameter the Finality Provider and BTC delegator Babylon
  address and the amount of satoshis from the rollbacked BTC undelegate.
  This will already update all the keys in the state accordingly.

#### 3.4 `MsgInsertHeaders`

The module `x/btclightclient` automatically handles the rollback of blocks, the issue
relies when the Babylon chain was halted while Bitcoin was producing new blocks, the
new blocks should have it's headers collected and included into the Babylon chain
state when it is being recovered. If the headers are not inserted, an attacker
could mine just 1-2 blocks on the forked Bitcoin chain and cause Babylon chain
to execute fake delegations, right after Babylon chain restarts. To avoid this,
Babylon should insert headers of the created Bitcoin blocks while it was
down during the recovery procedure.

#### 3.5 `MsgInsertBTCSpvProof`

All the proofs that were sent by the Vigilante Reporter in the Bitcoin blocks
that were affected by the rollback, will need to be revoked and might modify
the status of checkpoints. The recovery procedure for this specific case
might also need to modify the Vigilante to send again transactions of
checkpoints to the main chain of Bitcoin. This inserted proofs will
then need to be included into the Babylon state and checkpoint status updated
accordingly to the chain configuration of `BtcConfirmationDepth` and
`CheckpointFinalizationTimeout` in the `x/btccheckpoint` module.

### 4. Overall updates

Beside the specific cases of the messages sent, there is also the need
to update some states about the new heights as in:

- `x/btcstaking`
  - Update [`BTCHeightKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L25)
  the Babylon height corresponding to the BTC block height
  - Remove the [`LargestBtcReorgInBlocks`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L32)
  value previous set, to avoid halting again from the same BTC reorg.

### Create the emergency upgrade handler

The emergency upgrade handler is called a
[`Fork`](https://github.com/babylonlabs-io/babylon/blob/b56406b48b3d3b541c8aa57fe4490edb0fbff6a8/app/upgrades/types.go#L43) in the structures as the chain is halted and there is no possibility to create a software upgrade proposal
that nicely handles the upgrade plan. So, the fork structure
would contain as the name something that correlates with the BTC block heights
which were reorg, the upgrade height would be defined as the Babylon block
height in which the panic for reorg happened and the `BeginForkLogic`
should contain a single function that modifies the keepers with the data
collected during step [3](#3-analyze-each-message).

After that, tag a new release with the emergency upgrade in it, following
the [Release Procedure](../../../RELEASE_PROCESS.md#release-procedure). Test the logic in a private environment and if all the state is modified as expected, announce the new binary for Validators.
