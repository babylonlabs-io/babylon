package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// init epoch number
	if err := k.InitEpoch(ctx, gs.Epochs); err != nil {
		return err
	}
	// init msg queue
	if err := k.InitGenMsgQueue(ctx, gs.Queues); err != nil {
		return err
	}
	// init validator set
	if err := k.InitGenValidatorSet(ctx, gs.ValidatorSets); err != nil {
		return err
	}
	// init slashed voting power
	if err := k.InitGenSlashedVotingPower(ctx, gs.SlashedValidatorSets); err != nil {
		return err
	}

	// validators lifecycles
	for _, vl := range gs.ValidatorsLifecycle {
		valAddr, err := sdk.ValAddressFromBech32(vl.ValAddr)
		if err != nil {
			return err
		}
		k.SetValLifecycle(ctx, valAddr, vl)
	}

	// delegations lifecycles
	for _, dl := range gs.DelegationsLifecycle {
		k.SetDelegationLifecycle(ctx, sdk.MustAccAddressFromBech32(dl.DelAddr), dl)
	}

	// set params for this module
	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	es, err := k.epochs(ctx)
	if err != nil {
		return nil, err
	}

	epochsQueues := make([]*types.EpochQueue, 0)
	epochsValSet := make([]*types.EpochValidatorSet, 0)
	epochsSlashedValSet := make([]*types.EpochValidatorSet, 0)

	for _, e := range es {
		// queued message
		msgs := k.GetEpochMsgs(ctx, e.EpochNumber)
		if len(msgs) > 0 {
			epochMsgs := &types.EpochQueue{
				EpochNumber: e.EpochNumber,
				Msgs:        msgs,
			}
			epochsQueues = append(epochsQueues, epochMsgs)
		}

		// validator set
		vs := k.GetValidatorSet(ctx, e.EpochNumber)
		if len(vs) > 0 {
			epochValSet := &types.EpochValidatorSet{
				EpochNumber: e.EpochNumber,
			}
			for _, v := range vs {
				epochValSet.Validators = append(epochValSet.Validators, &v)
			}
			epochsValSet = append(epochsValSet, epochValSet)
		}

		// slashed validators set
		svs := k.GetSlashedValidators(ctx, e.EpochNumber)
		if len(svs) > 0 {
			epochSlashedValSet := &types.EpochValidatorSet{
				EpochNumber: e.EpochNumber,
			}
			for _, v := range svs {
				epochSlashedValSet.Validators = append(epochSlashedValSet.Validators, &v)
			}
			epochsSlashedValSet = append(epochsSlashedValSet, epochSlashedValSet)
		}
	}

	valsLc, err := k.validatorsLifecycle(ctx)
	if err != nil {
		return nil, err
	}
	delsLc, err := k.delegationsLifecycle(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:               k.GetParams(ctx),
		Epochs:               es,
		Queues:               epochsQueues,
		ValidatorSets:        epochsValSet,
		SlashedValidatorSets: epochsSlashedValSet,
		ValidatorsLifecycle:  valsLc,
		DelegationsLifecycle: delsLc,
	}, nil
}

func (k Keeper) epochs(ctx context.Context) ([]*types.Epoch, error) {
	epochs := make([]*types.Epoch, 0)
	store := k.epochInfoStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var epoch types.Epoch
		if err := k.cdc.Unmarshal(iter.Value(), &epoch); err != nil {
			return nil, err
		}
		epochs = append(epochs, &epoch)
	}

	return epochs, nil
}

func (k Keeper) validatorsLifecycle(ctx context.Context) ([]*types.ValidatorLifecycle, error) {
	lc := make([]*types.ValidatorLifecycle, 0)
	store := k.valLifecycleStore(ctx)

	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var valLc types.ValidatorLifecycle
		if err := k.cdc.Unmarshal(iter.Value(), &valLc); err != nil {
			return nil, err
		}
		lc = append(lc, &valLc)
	}

	return lc, nil
}

func (k Keeper) delegationsLifecycle(ctx context.Context) ([]*types.DelegationLifecycle, error) {
	lc := make([]*types.DelegationLifecycle, 0)
	store := k.delegationLifecycleStore(ctx)

	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var delLc types.DelegationLifecycle
		if err := k.cdc.Unmarshal(iter.Value(), &delLc); err != nil {
			return nil, err
		}
		lc = append(lc, &delLc)
	}

	return lc, nil
}
