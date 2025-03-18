# Large BTC Reorgs Recovery Procedure

Babylon relies in a number of confirmation blocks in BTC to accept BTC
staking transacitons, this number is defined inside the `x/btccheckpoint`
Parameter as `BtcConfirmationDepth` and as `k` in the research papers
which we considerate to be an irreversible block.

If for some unexpected reason there is a reorg in the bitcoin blockchain
larger than the `BtcConfirmationDepth`, the chain should halt and revoke all
the BTC delegations that were recorded until the first reorg block height.

By example, if BTC is in block height 150 and Babylon chain has a
`BtcConfirmationDepth` of 10 and it is submitted a valid BTC reorg of 15 blocks
that rollbacks BTC to block height 136, every BTC delegation that was included
in BTC block 136 or higher should be deleted as the inclusion proof will
be different. BTC delegations that included in the BTC block height 135 or lower
will not be affected as the BTC rollbacks only happens if there is a better
chain presented, a better chain means chain with more blocks, so any BTC
delegation included in block 135 or prior would still be considered valid.

This recovery procedure only cares about the BTC transactions that were executed
in blocks which have been rollbacked.

## Chain Halt

If the chain halted due to a BTC reorg largest than `BtcConfirmationDepth`,
there is a few steps to be followed to gather the data to be modified at the
emergency upgrade handler:

### 1. Collect correspondent Babylon height

The correlation of BTC and Babylon block heights can be found in the `x/btcstaking`
state in the [BlockHeightChains](https://github.com/babylonlabs-io/babylon/blob/fcc6fdc009e414da440426e6b81920ceef981de3/x/btcstaking/types/genesis.pb.go#L36),
currently there is no query for it, but it can be retrieved by exporting the
genesis state.

### 2. Collect BTC related msgs that were rollbacked

After having collected the babylon block height corresponded to the BTC block
height which the rollback happened, iterate over every transaction until
the halt block and gather all the messages which were approved and are one of
the types: `MsgCreateBTCDelegation`, `MsgAddBTCDelegationInclusionProof`,
`MsgBTCUndelegate`, `MsgInsertHeaders` and `MsgInsertBTCSpvProof`.

### 3. Analyze each message

Each BTC related msg that was rollbacked might need to have a different treatment.

#### 3.1 `MsgCreateBTCDelegation`

The msg which creates a new BTC delegation, if doesn't have inclusion proof
can stay as it is was not active or anything and neither had the inclusion
proof sent.

If there was a inclusion proof, verify if the proof is one from the BTC block
heights which was rollbacked, if it is from an BTC block that was not reorg,
but it was only later included in the Babylon chain, all good and there is
nothing to do. Now, if it was rollbacked this BTC staking transaction needs to
be removed from the following modules state `x/btcstaking`, `x/finality`,
`x/incentives`.

- `x/btcstaking`
  - Remove from [`BTCDelegatorKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L22)
  the BTC staking tx hash.
  - Remove from [`BTCDelegationKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L23)
  the BTC staking tx hash.
  - Remove the [`PowerDistUpdateKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L27)
  if a `EventPowerDistUpdate_BtcDelStateUpdate` was emitted for that
  BTC staking tx hash.
- `x/finality`
  - Update the voting power amount in [`VotingPowerKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L50)
  by subtracting the satoshi amounts of the finality provider that the BTC
  delegation delegated to if that BTC delegation was active.
  - Subtract the `TotalVotingPower` and the respective `FinalityProviderDistInfo`
  based on which finality provider was delegated to in
  [`VotingPowerDistCacheKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L51)
  if that BTC delegation was active.
- `x/incentives` If that BTC delegation was ever active, there is a need to
subtract the voting power from the rewards tracker. Also it is only modified
the values from now on, if rewards were accured from a BTC staking transaction
that was rollbacked, the past is behind us and it should be let with the funds
that were accured, losing a few coins in the rewards.
  - For the incentives module state there is a single function which should be
  called [`BtcDelegationUnbonded`](https://github.com/babylonlabs-io/babylon/blob/c8c44be12eb826b41f6f2cd3eae4452268398cdf/x/incentive/keeper/reward_tracker.go#L47)
  which receives as parameter the Finality provider and BTC delegator baby
  address and the amount of satoshi from the rollbacked BTC delegation.
  This will already update all the keys in the state accordingly.

> Note transactions that were included during blocks 150 ~ 141 (150 - `BtcConfirmationDepth`)
in a 15 Bitcoin blocks rollback are invalid and should be removed but were
also not  considered deep enough to be active. Dropping those proofs and
the event updates to voting power table is enough to handle during the
recory procedure.

#### 3.2 `MsgAddBTCDelegationInclusionProof`

Check if the proof is one from the BTC block heights which was rollbacked,
if it is not, there is no need to do anything. If it was rollbacked
the same steps defined in [3.1](#31-msgcreatebtcdelegation) need to be taken
to remove the data from the 3 modules states.

#### 3.3 `MsgBTCUndelegate`

Check if the inclusion proof had a reorg in the BTC blocks, if it is not
rollbacked, there is nothing to do. If it was rollbacked it is needed
to remove the existence of this BTC undelegation and affecting 3 modules state
with similar steps took at [3.1](#31-msgcreatebtcdelegation).

- `x/btcstaking`
  - Clean out the field for `BtcUndelegation` in `BTCDelegation` for the key
  [`BTCDelegationKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L23)
  in which corresponds to the BTC staking tx hash.
  - Remove the [`PowerDistUpdateKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L27)
  if a `EventPowerDistUpdate_BtcDelStateUpdate` was emitted for that
  BTC staking tx hash with Undelegate.
- `x/finality`
  - Update the voting power amount in [`VotingPowerKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L50)
  by adding the satoshi amounts of the finality provider that the BTC
  delegation delegated to.
  - Add the `TotalVotingPower` and the respective `FinalityProviderDistInfo`
  based on which finality provider was delegated to in
  [`VotingPowerDistCacheKey`](https://github.com/babylonlabs-io/babylon/blob/40f890d56d0bb081a6ce413281cc025f3d8b91d1/x/finality/types/keys.go#L51).
- `x/incentives` It is needed to add the voting power from the rewards tracker.
  - For the incentives module state there is a single function which should be
  called [`BtcDelegationActivated`](https://github.com/babylonlabs-io/babylon/blob/c8c44be12eb826b41f6f2cd3eae4452268398cdf/x/incentive/keeper/reward_tracker.go#L34)
  which receives as parameter the Finality provider and BTC delegator baby
  address and the amount of satoshi from the rollbacked BTC undelegate.
  This will already update all the keys in the state accordingly.

#### 3.4 `MsgInsertHeaders`

The module `x/btclightclient` automatically handles the rollback of blocks, the issue
relies when the Babylon chain was halted while Bitcoin was producing new blocks, the
new blocks should have it's headers collected and included into the Babylon chain
state when it is being recovered. If the headers are not insterted, an attacker
could mine just 1-2 blocks on the forked Bitcoin chain and cause Babylon chain
to execute fake delegations, right after Babylon chain restarts. To avoid this,
Babylon should insert headers of the created Bitcoind blocks while it was
down during the recovery procedure.

#### 3.5 `MsgInsertBTCSpvProof`

All the proofs that were sent by the vigilante reporter in the Bitcoin blocks
that were affected by the rollback, will need to be revoked and might modify
the status of checkpoints, the recovery procedure for this specific case
might also need to modify the vigilante to send again transactions of
checkpoints to the main chain of Bitcoin. This inserted proofs will
then need to be included into Babylon state and update checkpoint status
accordingly to chain configuration of `BtcConfirmationDepth` and
`CheckpointFinalizationTimeout` in the `x/btccheckpoint` module.

### 4. Overall updates

Beside the specific cases of the messages sent, there is also the need
to update some state about the new heights as in:

- `x/btcstaking`
  - Update [`BTCHeightKey`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L25)
  the babylon height corresponded to the BTC block height
  - Remove the [`LargestBtcReorgInBlocks`](https://github.com/babylonlabs-io/babylon/blob/7727f91491d5b8ddd6c10fa285ef3bea8a5ded4d/x/btcstaking/types/keys.go#L32)
  value previous set, to avoid halting again from the same BTC reorg.

### Create the emergency upgrade handler

The emergency upgrade handler is called a
[`Fork`](https://github.com/babylonlabs-io/babylon/blob/b56406b48b3d3b541c8aa57fe4490edb0fbff6a8/app/upgrades/types.go#L43) in the structures as the chain is halted
and there is no possibility to create a software upgrade proposal
that handles nicely the upgrade plan. So, the fork structure
would contain as the name something that correlates with the BTC block heights
which were reorg, the upgrade height would be defined as the Babylon block
height in which the panic for reorg happened and the `BeginForkLogic`
should contain a single function that modifies the keepers with the data
collected during step [3](#3-analyze-each-message).

After that, tag a new release with the emergency upgrade in it, following
the [Release Procedure](../../../RELEASE_PROCESS.md#release-procedure)
test the logic in a private enviroment and if it all the state is modified
as expected, announce the new binary for validators.
