package app

import (
	"encoding/json"
	"log"

	storetypes "cosmossdk.io/store/types"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ExportAppStateAndValidators exports the state of the application for a genesis
// file.
func (app *BabylonApp) ExportAppStateAndValidators(
	forZeroHeight bool,
	jailAllowedAddrs []string,
	modulesToExport []string) (servertypes.ExportedApp, error) {
	// as if they could withdraw from the start of the next block
	ctx := app.NewContext(true)

	// We export at last height + 1, because that's the height at which
	// Comet will start InitChain.
	height := app.LastBlockHeight() + 1
	if forZeroHeight {
		height = 0
		app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs)
	}

	genState, err := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.BaseApp.GetConsensusParams(ctx),
	}, err
}

// prepare for fresh start at zero height
// NOTE zero height genesis is a temporary feature which will be deprecated
// in favour of export at a block height
func (app *BabylonApp) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) {
	applyAllowedAddrs := false

	// check if there is a allowed address list
	if len(jailAllowedAddrs) > 0 {
		applyAllowedAddrs = true
	}

	allowedAddrsMap := make(map[string]bool)

	for _, addr := range jailAllowedAddrs {
		_, err := sdk.ValAddressFromBech32(addr)
		if err != nil {
			log.Fatal(err)
		}
		allowedAddrsMap[addr] = true
	}

	/* Handle fee distribution state. */

	// withdraw all validator commission
	err := app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			panic(err)
		}
		if _, err = app.DistrKeeper.WithdrawValidatorCommission(ctx, valBz); err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// withdraw all delegator rewards
	dels, err := app.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		panic(err)
	}
	for _, delegation := range dels {
		valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			panic(err)
		}

		delAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			panic(err)
		}
		if _, err = app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
	}

	// clear validator slash events
	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)

	// clear validator historical rewards
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// set context height to zero
	height := ctx.HeaderInfo().Height
	headerInfo := ctx.HeaderInfo()
	headerInfo.Height = 0
	ctx = ctx.WithHeaderInfo(headerInfo)

	// reinitialize all validators
	err = app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			panic(err)
		}
		// donate any unwithdrawn outstanding reward fraction tokens to the community pool
		scraps, err := app.DistrKeeper.GetValidatorOutstandingRewardsCoins(ctx, valBz)
		if err != nil {
			panic(err)
		}
		feePool, err := app.DistrKeeper.FeePool.Get(ctx)
		if err != nil {
			panic(err)
		}
		feePool.CommunityPool = feePool.CommunityPool.Add(scraps...)
		if err := app.DistrKeeper.FeePool.Set(ctx, feePool); err != nil {
			panic(err)
		}

		if err := app.DistrKeeper.Hooks().AfterValidatorCreated(ctx, sdk.ValAddress(val.GetOperator())); err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// reinitialize all delegations
	for _, del := range dels {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		delAddr := sdk.MustAccAddressFromBech32(del.DelegatorAddress)
		if err := app.DistrKeeper.Hooks().BeforeDelegationCreated(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
		if err := app.DistrKeeper.Hooks().AfterDelegationModified(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
	}

	// reset context height
	headerInfo = ctx.HeaderInfo()
	headerInfo.Height = height
	ctx = ctx.WithHeaderInfo(headerInfo)

	/* Handle staking state. */

	// iterate through redelegations, reset creation height
	err = app.StakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) (stop bool) {
		for i := range red.Entries {
			red.Entries[i].CreationHeight = 0
		}
		if err := app.StakingKeeper.SetRedelegation(ctx, red); err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// iterate through unbonding delegations, reset creation height
	err = app.StakingKeeper.IterateUnbondingDelegations(ctx, func(_ int64, ubd stakingtypes.UnbondingDelegation) (stop bool) {
		for i := range ubd.Entries {
			ubd.Entries[i].CreationHeight = 0
		}
		if err := app.StakingKeeper.SetUnbondingDelegation(ctx, ubd); err != nil {
			panic(err)
		}
		return false
	})
	if err != nil {
		panic(err)
	}

	// Iterate through validators by power descending, reset bond heights, and
	// update bond intra-tx counters.
	store := ctx.KVStore(app.GetKey(stakingtypes.StoreKey))
	iter := storetypes.KVStoreReversePrefixIterator(store, stakingtypes.ValidatorsKey)

	for ; iter.Valid(); iter.Next() {
		addr := sdk.ValAddress(stakingtypes.AddressFromValidatorsKey(iter.Key()))
		validator, err := app.StakingKeeper.GetValidator(ctx, addr)
		if err != nil {
			panic("expected validator, not found")
		}

		validator.UnbondingHeight = 0
		if applyAllowedAddrs && !allowedAddrsMap[addr.String()] {
			validator.Jailed = true
		}

		if err := app.StakingKeeper.SetValidator(ctx, validator); err != nil {
			panic(err)
		}
	}

	if err := iter.Close(); err != nil {
		app.Logger().Error("error while closing the key-value store reverse prefix iterator: ", err)
		return
	}

	_, err = app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	if err != nil {
		log.Fatal(err)
	}

	/* Handle slashing state. */

	// reset start height on signing infos
	err = app.SlashingKeeper.IterateValidatorSigningInfos(
		ctx,
		func(addr sdk.ConsAddress, info slashingtypes.ValidatorSigningInfo) (stop bool) {
			info.StartHeight = 0
			if err := app.SlashingKeeper.SetValidatorSigningInfo(ctx, addr, info); err != nil {
				panic(err)
			}
			return false
		},
	)
	if err != nil {
		panic(err)
	}
}
