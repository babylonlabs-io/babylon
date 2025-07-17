package keeper

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"cosmossdk.io/store/prefix"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

// CreateBTCDelegation creates a BTC delegation
func (k Keeper) CreateBTCDelegation(ctx sdk.Context, parsedMsg *types.ParsedCreateDelegationMessage) error {
	// 1. sanity check the parsed msg
	if parsedMsg == nil {
		return status.Error(codes.InvalidArgument, "parsed create delegation message is nil")
	}

	signingContext := signingcontext.StakerPopContextV0(ctx.ChainID(), k.btcStakingModuleAddress)

	// 2. Basic stateless checks
	// - verify proof of possession
	if err := parsedMsg.ParsedPop.Verify(signingContext, parsedMsg.StakerAddress, parsedMsg.StakerPK.BIP340PubKey, k.btcNet); err != nil {
		return types.ErrInvalidProofOfPossession.Wrap(err.Error())
	}

	// 3. Check if it is not duplicated staking tx
	stakingTxHash := parsedMsg.StakingTx.Transaction.TxHash()
	delegation := k.getBTCDelegation(ctx, stakingTxHash)
	if delegation != nil {
		return types.ErrReusedStakingTx.Wrapf("duplicated tx hash: %s", stakingTxHash.String())
	}

	// 5. Get params for the validated inclusion height either tip or inclusion height
	timeInfo, params, paramsVersion, err := k.getTimeInfoAndParams(ctx, parsedMsg)
	if err != nil {
		return err
	}

	// 6. Validate the staking tx against the params
	paramsValidationResult, err := types.ValidateParsedMessageAgainstTheParams(parsedMsg, params, k.btcNet)
	if err != nil {
		return err
	}

	// Ensure all finality providers
	// - are known to Babylon
	// - every FP is from different BSN
	// - exactly one FP is from Babylon
	if err := k.validateMultiStakedFPs(ctx, parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat); err != nil {
		return err
	}

	// 7. if allow list is enabled we need to check whether staking transactions hash
	// is in the allow list
	if isAllowListEnabled(ctx, params) {
		if !k.IsStakingTransactionAllowed(ctx, &stakingTxHash) {
			return types.ErrInvalidStakingTx.Wrapf("staking tx hash: %s, is not in the allow list", stakingTxHash.String())
		}
	}

	// Check multi-staking allow list
	// During multi-staking allow-list period, only existing BTC delegations
	// in the allow-list can become multi-staked via stake expansion or
	// already existing multi-staking delegation (extended from the allow-list)
	isMultiStaking := len(parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat) > 1
	if isMultiStaking && types.IsMultiStakingAllowListEnabled(ctx.BlockHeight()) {
		// if is not stake expansion, it is not allowed to create new delegations with multi-staking
		if parsedMsg.StkExp == nil {
			return types.ErrInvalidStakingTx.Wrap("it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period")
		}

		// if it is stake expansion, we need to check if the previous staking tx hash
		// is in the allow list or the previous staking tx is a multi-staking tx
		allowed, err := k.IsMultiStakingAllowed(ctx, parsedMsg.StkExp.PreviousActiveStkTxHash)
		if err != nil {
			return fmt.Errorf("failed to check if the previous staking tx hash is elegible for multi-staking: %w", err)
		}
		if !allowed {
			return types.ErrInvalidStakingTx.Wrapf("staking tx hash: %s, is not elegible for multi-staking", parsedMsg.StkExp.PreviousActiveStkTxHash.String())
		}
	}

	// everything is good, if the staking tx is not included on BTC consume additinal
	// gas
	if !parsedMsg.IsIncludedOnBTC() {
		ctx.GasMeter().ConsumeGas(params.DelegationCreationBaseGasFee, "delegation creation fee")
	}

	// 7.all good, construct BTCDelegation and insert BTC delegation
	// NOTE: the BTC delegation does not have voting power yet. It will
	// have voting power only when it receives a covenant signatures
	newBTCDel := &types.BTCDelegation{
		StakerAddr:       parsedMsg.StakerAddress.String(),
		BtcPk:            parsedMsg.StakerPK.BIP340PubKey,
		Pop:              parsedMsg.ParsedPop,
		FpBtcPkList:      parsedMsg.FinalityProviderKeys.PublicKeysBbnFormat,
		StakingTime:      uint32(parsedMsg.StakingTime),
		StartHeight:      timeInfo.StartHeight,
		EndHeight:        timeInfo.EndHeight,
		TotalSat:         uint64(parsedMsg.StakingValue),
		StakingTx:        parsedMsg.StakingTx.TransactionBytes,
		StakingOutputIdx: paramsValidationResult.StakingOutputIdx,
		SlashingTx:       types.NewBtcSlashingTxFromBytes(parsedMsg.StakingSlashingTx.TransactionBytes),
		DelegatorSig:     parsedMsg.StakerStakingSlashingTxSig.BIP340Signature,
		UnbondingTime:    uint32(parsedMsg.UnbondingTime),
		CovenantSigs:     nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
		BtcUndelegation: &types.BTCUndelegation{
			UnbondingTx:              parsedMsg.UnbondingTx.TransactionBytes,
			SlashingTx:               types.NewBtcSlashingTxFromBytes(parsedMsg.UnbondingSlashingTx.TransactionBytes),
			DelegatorSlashingSig:     parsedMsg.StakerUnbondingSlashingSig.BIP340Signature,
			CovenantSlashingSigs:     nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
			CovenantUnbondingSigList: nil, // NOTE: covenant signature will be submitted in a separate msg by covenant
			DelegatorUnbondingInfo:   nil,
		},
		ParamsVersion: paramsVersion,      // version of the params against which delegation was validated
		BtcTipHeight:  timeInfo.TipHeight, // height of the BTC light client tip at the time of the delegation creation
	}

	newBTCDel.StkExp, err = buildStakeExpansion(parsedMsg.StkExp)
	if err != nil {
		return fmt.Errorf("error building stake expansion: %w", err)
	}

	// add this BTC delegation, and emit corresponding events
	if err := k.AddBTCDelegation(ctx, newBTCDel); err != nil {
		return fmt.Errorf("failed to add BTC delegation that has passed verification: %w", err)
	}

	return nil
}

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

	// for each finality provider the delegation multi-stakes to, update its index
	for _, fpBTCPK := range btcDel.FpBtcPkList {
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
	delInfo *btcDelegationWithParams,
	covPK *bbn.BIP340PubKey,
	parsedSlashingAdaptorSignatures []asig.AdaptorSignature,
	unbondingTxSig *bbn.BIP340Signature,
	parsedUnbondingSlashingAdaptorSignatures []asig.AdaptorSignature,
	stakeExpansionTxSig *bbn.BIP340Signature,
	btcTipHeight uint32,
) {
	btcDel := delInfo.Delegation
	var quorumPreviousStk uint32
	if btcDel.IsStakeExpansion() {
		// if this is stake expansion, we need to get the previous staking tx quorum
		quorumPreviousStk = delInfo.PrevParams.CovenantQuorum
	}

	hadQuorum := btcDel.HasCovenantQuorums(delInfo.Params.CovenantQuorum, quorumPreviousStk)

	// All is fine add received signatures to the BTC delegation and BtcUndelegation
	btcDel.AddCovenantSigs(
		covPK,
		parsedSlashingAdaptorSignatures,
		unbondingTxSig,
		parsedUnbondingSlashingAdaptorSignatures,
		stakeExpansionTxSig,
	)

	// set BTC delegation back to KV store
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
	if !hadQuorum && btcDel.HasCovenantQuorums(delInfo.Params.CovenantQuorum, quorumPreviousStk) {
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
			k.addPowerDistUpdateEvent(ctx, btcTipHeight, activeEvent)

			// notify consumer chains about the active BTC delegation
			k.notifyConsumersOnActiveBTCDel(ctx, btcDel)
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

func (k Keeper) notifyConsumersOnActiveBTCDel(ctx context.Context, btcDel *types.BTCDelegation) {
	// get consumer ids of only non-Babylon finality providers
	multiStakedFPConsumerIDs, err := k.multiStakedFPConsumerIDs(ctx, btcDel.FpBtcPkList)
	if err != nil {
		panic(fmt.Errorf("failed to get consumer ids for the multi-staked BTC delegation: %w", err))
	}
	consumerEvent, err := types.CreateActiveBTCDelegationEvent(btcDel)
	if err != nil {
		panic(fmt.Errorf("failed to create active BTC delegation event: %w", err))
	}
	for _, consumerID := range multiStakedFPConsumerIDs {
		if err := k.AddBTCStakingConsumerEvent(ctx, consumerID, consumerEvent); err != nil {
			panic(fmt.Errorf("failed to add active BTC delegation event: %w", err))
		}
	}
}

// btcUndelegate adds the signature of the unbonding tx signed by the staker
// to the given BTC delegation
func (k Keeper) btcUndelegate(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	u *types.DelegatorUnbondingInfo,
	stakeSpendingTx []byte,
	proof *types.InclusionProof,
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

	// get consumer ids of only non-Babylon finality providers
	multiStakedFPConsumerIDs, err := k.multiStakedFPConsumerIDs(ctx, btcDel.FpBtcPkList)
	if err != nil {
		panic(fmt.Errorf("failed to get consumer ids for the multi-staked BTC delegation: %w", err))
	}
	// create consumer event for unbonded BTC delegation and add it to the consumer's event store
	consumerEvent, err := types.CreateUnbondedBTCDelegationEvent(btcDel, stakeSpendingTx, proof)
	if err != nil {
		panic(fmt.Errorf("failed to create unbonded BTC delegation event: %w", err))
	}
	for _, consumerID := range multiStakedFPConsumerIDs {
		if err = k.AddBTCStakingConsumerEvent(ctx, consumerID, consumerEvent); err != nil {
			panic(fmt.Errorf("failed to add active BTC delegation event: %w", err))
		}
	}
}

// isAllowListEnabled checks if the allow list is enabled at the given height
// allow list is enabled if AllowListExpirationHeight is larger than 0,
// and current block height is less than AllowListExpirationHeight
func isAllowListEnabled(ctx sdk.Context, p *types.Params) bool {
	return p.AllowListExpirationHeight > 0 && uint64(ctx.BlockHeight()) < p.AllowListExpirationHeight
}

func (k Keeper) getTimeInfoAndParams(
	ctx sdk.Context,
	parsedMsg *types.ParsedCreateDelegationMessage,
) (*DelegationTimeRangeInfo, *types.Params, uint32, error) {
	if parsedMsg.IsIncludedOnBTC() {
		// staking tx is already included on BTC
		// 1. Validate inclusion proof and retrieve inclusion height
		// 2. Get params for the validated inclusion height
		btccParams := k.btccKeeper.GetParams(ctx)

		timeInfo, err := k.VerifyInclusionProofAndGetHeight(
			ctx,
			btcutil.NewTx(parsedMsg.StakingTx.Transaction),
			btccParams.BtcConfirmationDepth,
			uint32(parsedMsg.StakingTime),
			uint32(parsedMsg.UnbondingTime),
			parsedMsg.StakingTxProofOfInclusion,
		)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid inclusion proof: %w", err)
		}

		paramsByHeight, version, err := k.GetParamsForBtcHeight(ctx, uint64(timeInfo.StartHeight))
		if err != nil {
			// this error can happen if we receive delegations which is included before
			// first activation height we support
			return nil, nil, 0, err
		}

		return timeInfo, paramsByHeight, version, nil
	}
	// staking tx is not included on BTC, retrieve params for the current tip height
	// and return info about the tip
	btcTip := k.btclcKeeper.GetTipInfo(ctx)

	paramsByHeight, version, err := k.GetParamsForBtcHeight(ctx, uint64(btcTip.Height))
	if err != nil {
		return nil, nil, 0, err
	}

	return &DelegationTimeRangeInfo{
		StartHeight: 0,
		EndHeight:   0,
		TipHeight:   btcTip.Height,
	}, paramsByHeight, version, nil
}

func (k Keeper) setBTCDelegation(ctx context.Context, btcDel *types.BTCDelegation) {
	store := k.btcDelegationStore(ctx)
	stakingTxHash := btcDel.MustGetStakingTxHash()
	btcDelBytes := k.cdc.MustMarshal(btcDel)
	store.Set(stakingTxHash[:], btcDelBytes)
}

// validateMultiStakedFPs ensures all finality providers
// - are known to Babylon
// - exactly one of them is a Babylon finality provider
// - all consumer finality providers are from different consumer chains
func (k Keeper) validateMultiStakedFPs(ctx sdk.Context, fpBTCPKs []bbn.BIP340PubKey) error {
	fpConsumerCounters := make(map[string]int)
	babylonFpCount := 0

	for i := range fpBTCPKs {
		fpBTCPK := fpBTCPKs[i]

		fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
		if err != nil {
			return types.ErrFpNotFound.Wrapf("finality provider pk %s is not found: %v", fpBTCPK.MarshalHex(), err)
		}
		if fp.IsSlashed() {
			return types.ErrFpAlreadySlashed
		}

		if fp.SecuresBabylonGenesis(ctx) {
			babylonFpCount++

			if babylonFpCount > 1 {
				return types.ErrInvalidMultiStakingFPs.Wrap("more than one Babylon finality provider found in the multi-staking selection")
			}
		} else {
			fpConsumerCounters[fp.BsnId]++
			if fpConsumerCounters[fp.BsnId] > 1 {
				return types.ErrInvalidMultiStakingFPs.Wrapf("more than one finality provider found from the same BSN: %s, in the multi-staking selection", fp.BsnId)
			}
		}
	}

	if babylonFpCount == 0 {
		// a BTC delegation has to stake to at least 1 Babylon finality provider
		return types.ErrNoBabylonFPMultiStaked
	}

	return nil
}

// multiStakedFPConsumerIDs returns the unique consumer IDs of non-Babylon finality providers
// The returned list is sorted in order to make sure the function is deterministic
func (k Keeper) multiStakedFPConsumerIDs(ctx context.Context, fpBTCPKs []bbn.BIP340PubKey) ([]string, error) {
	consumerIDMap := make(map[string]struct{})

	for i := range fpBTCPKs {
		fpBTCPK := fpBTCPKs[i]
		fp, err := k.GetFinalityProvider(ctx, fpBTCPK)
		if err != nil {
			return nil, err
		}
		if !fp.SecuresBabylonGenesis(sdk.UnwrapSDKContext(ctx)) {
			consumerIDMap[fp.BsnId] = struct{}{}
		}
	}

	uniqueConsumerIDs := make([]string, 0, len(consumerIDMap))
	for consumerID := range consumerIDMap {
		uniqueConsumerIDs = append(uniqueConsumerIDs, consumerID)
	}

	// Sort consumer IDs for deterministic ordering
	slices.Sort(uniqueConsumerIDs)

	return uniqueConsumerIDs, nil
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

// IsBtcDelegationActive returns true and no error if the BTC delegation is active.
// If it is not active it returns false and the reason as an error
func (k Keeper) IsBtcDelegationActive(ctx context.Context, stakingTxHash string) (*btcDelegationWithParams, bool, error) {
	delWithParams, err := k.getBTCDelWithParams(ctx, stakingTxHash)
	if err != nil {
		return nil, false, err
	}

	status, _, err := k.BtcDelStatusWithTip(ctx, delWithParams)
	if err != nil {
		return nil, false, err
	}

	if status != types.BTCDelegationStatus_ACTIVE {
		return nil, false, fmt.Errorf("BTC delegation %s is not active, current status is %s", stakingTxHash, status.String())
	}

	return delWithParams, true, nil
}

type btcDelegationWithParams struct {
	Delegation *types.BTCDelegation
	Params     *types.Params
	// Stake expansion fields
	PrevDel    *types.BTCDelegation // previous BTC delegation for stake expansion
	PrevParams *types.Params        // params of the previous BTC delegation for stake expansion
}

func (k Keeper) getBTCDelWithParams(
	ctx context.Context,
	stakingTxHash string,
) (*btcDelegationWithParams, error) {
	btcDel, err := k.GetBTCDelegation(ctx, stakingTxHash)
	if err != nil {
		return nil, err
	}

	bsParams := k.GetParamsByVersion(ctx, btcDel.ParamsVersion)
	if bsParams == nil {
		return nil, errors.New("params version in BTC delegation is not found")
	}

	res := &btcDelegationWithParams{Delegation: btcDel, Params: bsParams}
	if !btcDel.IsStakeExpansion() {
		return res, nil
	}

	// get stake expansion params and delegation
	stkExpInfo, err := k.getBTCDelWithParams(ctx, btcDel.MustGetStakeExpansionTxHash().String())
	if err != nil {
		return nil, fmt.Errorf("failed to get previous BTC delegation for stake expansion %s: %w", btcDel.MustGetStakeExpansionTxHash().String(), err)
	}

	res.PrevDel = stkExpInfo.Delegation
	res.PrevParams = stkExpInfo.Params

	return res, nil
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

func (k Keeper) BtcDelStatusWithTip(
	ctx context.Context,
	delInfo *btcDelegationWithParams,
) (status types.BTCDelegationStatus, btcTip *btclctypes.BTCHeaderInfo, err error) {
	if delInfo == nil || delInfo.Delegation == nil {
		return 0, nil, errors.New("BTC delegation is nil")
	}

	btcTip = k.btclcKeeper.GetTipInfo(ctx)
	// in case is stake expansion, we need to get the previous staking tx quorum
	// to get the delegation status
	var prevStkCovenantQuorum uint32
	if delInfo.Delegation.IsStakeExpansion() {
		prevStkCovenantQuorum = delInfo.PrevParams.CovenantQuorum
	}
	status = delInfo.Delegation.GetStatus(
		btcTip.Height,
		delInfo.Params.CovenantQuorum,
		prevStkCovenantQuorum,
	)
	return status, btcTip, err
}

func (k Keeper) BtcDelStatus(
	ctx context.Context,
	btcDel *types.BTCDelegation,
	covenantQuorum uint32,
	btcTipHeight uint32,
) (status types.BTCDelegationStatus, err error) {
	var quorumPreviousStk uint32
	if btcDel.IsStakeExpansion() {
		delInfo, err := k.getBTCDelWithParams(ctx, btcDel.MustGetStakeExpansionTxHash().String())
		if err != nil {
			return 0, err
		}
		quorumPreviousStk = delInfo.Params.CovenantQuorum
	}
	return btcDel.GetStatus(
		btcTipHeight,
		covenantQuorum,
		quorumPreviousStk,
	), nil
}

func (k Keeper) BtcDelHasCovenantQuorums(
	ctx context.Context,
	btcDel *types.BTCDelegation,
	quorum uint32,
) (hasQuorum bool, err error) {
	var quorumPreviousStk uint32
	if btcDel.IsStakeExpansion() {
		delInfo, err := k.getBTCDelWithParams(ctx, btcDel.MustGetStakeExpansionTxHash().String())
		if err != nil {
			return false, err
		}
		quorumPreviousStk = delInfo.Params.CovenantQuorum
	}
	return btcDel.HasCovenantQuorums(quorum, quorumPreviousStk), nil
}

func buildStakeExpansion(stkExp *types.ParsedCreateDelStkExp) (*types.StakeExpansion, error) {
	if stkExp == nil {
		return nil, nil
	}

	fundingOut, err := stkExp.SerializeOtherFundingOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx out for other funding output: %w", err)
	}

	return &types.StakeExpansion{
		PreviousStakingTxHash:   stkExp.PreviousActiveStkTxHash[:],
		OtherFundingTxOut:       fundingOut,
		PreviousStkCovenantSigs: nil,
	}, nil
}
