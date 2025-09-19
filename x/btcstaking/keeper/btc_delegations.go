package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	asig "github.com/babylonlabs-io/babylon/v4/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// AddBTCDelegation adds a BTC delegation post verification to the system, including
// - indexing the given BTC delegation in the BTC delegator store,
// - saving it under BTC delegation store, and
// - emit events about this BTC delegation.
func (k Keeper) AddBTCDelegation(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
) error {
	// get staking tx hash
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}

	// for each finality provider the delegation restakes to, update its index
	for _, fpBTCPK := range btcDel.FpBtcPkList {
		fpBTCPK := fpBTCPK // remove when update to go1.22
		// get BTC delegation index under this finality provider
		btcDelIndex := k.getBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk)
		if btcDelIndex == nil {
			btcDelIndex = types.NewBTCDelegatorDelegationIndex()
		}
		// index staking tx hash of this BTC delegation
		if err := btcDelIndex.Add(stakingTxHash); err != nil {
			return types.ErrInvalidStakingTx.Wrapf("error adding staking tx hash to BTC delegator index: %s", err.Error())
		}
		// save the index
		k.setBTCDelegatorDelegationIndex(ctx, &fpBTCPK, btcDel.BtcPk, btcDelIndex)
	}

	// save this BTC delegation
	k.setBTCDelegation(ctx, btcDel)

	if err := ctx.EventManager().EmitTypedEvents(types.NewBtcDelCreationEvent(
		btcDel,
	)); err != nil {
		panic(fmt.Errorf("failed to emit events for the new pending BTC delegation: %w", err))
	}

	// NOTE: we don't need to record events for pending BTC delegations since these
	// do not affect voting power distribution
	// NOTE: we only insert unbonded event if the delegation already has inclusion proof
	if btcDel.HasInclusionProof() {
		if err := ctx.EventManager().EmitTypedEvent(types.NewInclusionProofEvent(
			stakingTxHash.String(),
			btcDel.StartHeight,
			btcDel.EndHeight,
			types.BTCDelegationStatus_PENDING,
		)); err != nil {
			panic(fmt.Errorf("failed to emit EventBTCDelegationInclusionProofReceived for the new pending BTC delegation: %w", err))
		}

		// record event that the BTC delegation will become expired (unbonded) at EndHeight-w
		// This event will be generated to subscribers as block event, when the
		// btc light client block height will reach btcDel.EndHeight-wValue
		expiredEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
			StakingTxHash: stakingTxHash.String(),
			NewState:      types.BTCDelegationStatus_EXPIRED,
		})

		// NOTE: we should have verified that EndHeight > btcTip.Height + unbonding_time
		k.addPowerDistUpdateEvent(ctx, btcDel.EndHeight-btcDel.UnbondingTime, expiredEvent)
	}

	return nil
}

// addCovenantSigsToBTCDelegation adds signatures from a given covenant member
// to the given BTC delegation
func (k Keeper) addCovenantSigsToBTCDelegation(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	covPK *bbn.BIP340PubKey,
	parsedSlashingAdaptorSignatures []asig.AdaptorSignature,
	unbondingTxSig *bbn.BIP340Signature,
	parsedUnbondingSlashingAdaptorSignatures []asig.AdaptorSignature,
	params *types.Params,
) {
	hadQuorum := btcDel.HasCovenantQuorums(params.CovenantQuorum)

	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	btcDel.AddCovenantSigs(
		covPK,
		parsedSlashingAdaptorSignatures,
		unbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
	)

	k.setBTCDelegation(ctx, btcDel)

	if err := ctx.EventManager().EmitTypedEvent(types.NewCovenantSignatureReceivedEvent(
		btcDel,
		covPK,
		unbondingTxSig,
	)); err != nil {
		panic(fmt.Errorf("failed to emit EventCovenantSignatureRecevied for the new active BTC delegation: %w", err))
	}

	// If reaching the covenant quorum after this msg, the BTC delegation becomes
	// active. Then, record and emit this event
	// We only emit power distribution events, and external quorum events if it
	// is the first time the quorum is reached
	if !hadQuorum && btcDel.HasCovenantQuorums(params.CovenantQuorum) {
		if btcDel.HasInclusionProof() {
			quorumReachedEvent := types.NewCovenantQuorumReachedEvent(
				btcDel,
				types.BTCDelegationStatus_ACTIVE,
			)
			if err := ctx.EventManager().EmitTypedEvent(quorumReachedEvent); err != nil {
				panic(fmt.Errorf("failed to emit emit for the new verified BTC delegation: %w", err))
			}

			// record event that the BTC delegation becomes active at this height
			activeEvent := types.NewEventPowerDistUpdateWithBTCDel(
				&types.EventBTCDelegationStateUpdate{
					StakingTxHash: btcDel.MustGetStakingTxHash().String(),
					NewState:      types.BTCDelegationStatus_ACTIVE,
				},
			)
			btcTip := k.btclcKeeper.GetTipInfo(ctx)
			k.addPowerDistUpdateEvent(ctx, btcTip.Height, activeEvent)
		} else {
			quorumReachedEvent := types.NewCovenantQuorumReachedEvent(
				btcDel,
				types.BTCDelegationStatus_VERIFIED,
			)

			if err := ctx.EventManager().EmitTypedEvent(quorumReachedEvent); err != nil {
				panic(fmt.Errorf("failed to emit emit for the new verified BTC delegation: %w", err))
			}
		}
	}
}

// btcUndelegate adds the signature of the unbonding tx signed by the staker
// to the given BTC delegation
func (k Keeper) btcUndelegate(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	u *types.DelegatorUnbondingInfo,
) {
	btcDel.BtcUndelegation.DelegatorUnbondingInfo = u
	k.setBTCDelegation(ctx, btcDel)

	if !btcDel.HasInclusionProof() {
		return
	}

	// notify subscriber about this unbonded BTC delegation
	event := &types.EventBTCDelegationStateUpdate{
		StakingTxHash: btcDel.MustGetStakingTxHash().String(),
		NewState:      types.BTCDelegationStatus_UNBONDED,
	}

	// record event that the BTC delegation becomes unbonded at this height
	unbondedEvent := types.NewEventPowerDistUpdateWithBTCDel(event)
	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	k.addPowerDistUpdateEvent(ctx, btcTip.Height, unbondedEvent)
}

func (k Keeper) setBTCDelegation(ctx context.Context, btcDel *types.BTCDelegation) {
	store := k.btcDelegationStore(ctx)
	stakingTxHash := btcDel.MustGetStakingTxHash()
	btcDelBytes := k.cdc.MustMarshal(btcDel)
	store.Set(stakingTxHash[:], btcDelBytes)
}

// GetBTCDelegation gets the BTC delegation with a given staking tx hash
func (k Keeper) GetBTCDelegation(ctx context.Context, stakingTxHashStr string) (*types.BTCDelegation, error) {
	// decode staking tx hash string
	stakingTxHash, err := chainhash.NewHashFromStr(stakingTxHashStr)
	if err != nil {
		return nil, err
	}
	btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
	if btcDel == nil {
		return nil, types.ErrBTCDelegationNotFound
	}

	return btcDel, nil
}

func (k Keeper) getBTCDelegation(ctx context.Context, stakingTxHash chainhash.Hash) *types.BTCDelegation {
	store := k.btcDelegationStore(ctx)
	btcDelBytes := store.Get(stakingTxHash[:])
	if len(btcDelBytes) == 0 {
		return nil
	}
	var btcDel types.BTCDelegation
	k.cdc.MustUnmarshal(btcDelBytes, &btcDel)
	return &btcDel
}

// btcDelegationStore returns the KVStore of the BTC delegations
// prefix: BTCDelegationKey
// key: BTC delegation's staking tx hash
// value: BTCDelegation
func (k Keeper) btcDelegationStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.BTCDelegationKey)
}
