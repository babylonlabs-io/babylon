package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/x/btcdistribution/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// add 8 precisions point
var decimals_precision = math.NewInt(1_00000000)

type Keeper struct {
	cdc          codec.BinaryCodec
	btcStkK      types.BTCStakingKeeper
	stkK         types.StakingKeeper
	storeService corestoretypes.KVStoreService
}

func NewKeeper(
	btcStkK types.BTCStakingKeeper,
	stkK types.StakingKeeper,
	storeService corestoretypes.KVStoreService,
	cdc codec.BinaryCodec,
) Keeper {
	return Keeper{
		cdc:          cdc,
		btcStkK:      btcStkK,
		stkK:         stkK,
		storeService: storeService,
	}
}

func (k Keeper) RewardsForCurrentBlock() sdk.Coins {
	return sdk.NewCoins(
		sdk.NewCoin(appparams.BaseCoinUnit, math.NewInt(10_000000)),
	)
}

func (k Keeper) EndBlocker(ctx context.Context) error {
	protocolBtcStaked, err := k.btcStkK.TotalSatoshiStaked(ctx)
	if err != nil {
		return err
	}

	if !protocolBtcStaked.IsPositive() {
		return fmt.Errorf("invalid btc staked amount")
	}

	protocolNativeStaked, err := k.stkK.TotalBondedTokens(ctx)
	if err != nil {
		return err
	}

	if !protocolNativeStaked.IsPositive() {
		return fmt.Errorf("invalid native staked amount")
	}

	// C_btc = S_btc / S_btc_total
	// C_bbn = S_bbn / S_bbn_total
	// w = S_btc * g(C_bbn / C_btc)

	/// table test with 2 dels

	// del1 7 btc 20 bbn
	// del2 3 btc 80 bbn
	// total 10 btc 100 bbn

	// del1 S_btc / S_btc_total = 7/10 = 0.7
	// del1 S_bbn / S_bbn_total = 20/100 = 0.2
	// del1 C_btc = 0.7, C_bbn = 0.2
	// del1 g(C_bbn / C_btc) = 0.2 / 0.7 = 0.28571429
	// del1 S_btc * g(C_bbn / C_btc) = 7 * 0.28571429 = 2

	// del2 S_btc / S_btc_total = 3/10 = 0.3
	// del2 S_bbn / S_bbn_total = 80/100 = 0.8
	// del2 C_btc = 0.3, C_bbn = 0.8
	// del2 g(C_bbn / C_btc) = 0.8 / 0.3 = 2.66666666
	// del2 S_btc * g(C_bbn / C_btc) = 3 * 2.66666666 = 8

	/// table test with 3 dels

	// del1 7 btc 20 bbn
	// del2 3 btc 80 bbn
	// del3 15 btc 10 bbn
	// total 25 btc 110 bbn

	// del1 C_btc = S_btc / S_btc_total = 7/25 = 0,28
	// del1 C_bbn = S_bbn / S_bbn_total = 20/110 = 0,18181818
	// del1 C_btc = 0.28, C_bbn = 0,18181818
	// del1 g(C_bbn / C_btc) = 0,18181818 / 0,2 = 0,90909091
	// del1 S_btc * g(C_bbn / C_btc) = 7 * 0,90909091 = 6,36363636

	// 10_000000ubbn => 25
	// x 						 => 6,36363636
	// x = (6,36363636 * 10bbn) / 25
	//

	// del2 C_btc = S_btc / S_btc_total = 3/25 = 0,12
	// del2 C_bbn = S_bbn / S_bbn_total = 80/110 = 0,72727273
	// del2 C_btc = 0,12, C_bbn = 0,72727273
	// del2 g(C_bbn / C_btc) = 0,72727273 / 0.12 = 6,06060606
	// del2 S_btc * g(C_bbn / C_btc) = 3 * 6,06060606 = 18,18181818

	// del3 C_btc = S_btc / S_btc_total = 15/25 = 0,6
	// del3 C_bbn = S_bbn / S_bbn_total = 10/110 = 0,09090909
	// del3 C_btc = 0,6, C_bbn = 0,09090909
	// del3 g(C_bbn / C_btc) = 0,09090909 / 0.6 = 0,15151515
	// del3 S_btc * g(C_bbn / C_btc) = 3 * 0,15151515 = 0,45454545

	// Total weight is always the total amount of BTC staked

	// fake total rewards per block
	totalRewards := k.RewardsForCurrentBlock()
	err = k.btcStkK.IterateBTCDelegators(ctx, func(del sdk.AccAddress, delBtcStaked math.Int) error {
		delNativeStaked, err := k.stkK.GetDelegatorBonded(ctx, del)
		if err != nil {
			return nil
		}

		delNativeStaked = addDecimals(delNativeStaked)
		delBtcStaked = addDecimals(delBtcStaked)
		weight := weightStaked(protocolNativeStaked, protocolBtcStaked, delNativeStaked, delBtcStaked)
		if !weight.IsPositive() {
			return nil
		}

		weight = rmvDecimals(weight)
		rewards := rewardRatio(totalRewards, protocolBtcStaked, weight)
		return k.AcumulateDelRewards(ctx, del, rewards)
	})
	if err != nil {
		return err
	}

	return nil
}

// C_btc = S_btc / S_btc_total
// C_bbn = S_bbn / S_bbn_total
// weightStaked
func weightStaked(
	totalNativeStaked, totalBtcStaked math.Int,
	delNativeStaked, delBtcStaked math.Int,
) math.Int {
	ratioNativeDelToTotal := delNativeStaked.Quo(totalNativeStaked)
	if !totalBtcStaked.IsPositive() {
		return math.NewInt(0)
	}

	ratioBtcDelToTotal := delBtcStaked.Quo(totalBtcStaked)
	if !ratioNativeDelToTotal.IsPositive() {
		return math.NewInt(0)
	}
	return ratioBtcDelToTotal.Quo(ratioNativeDelToTotal)
	// return totalBtcStaked.Mul(ratioTotals)
}

func rewardRatio(totalRewards sdk.Coins, totalWeight, delWeight math.Int) sdk.Coins {
	// totalRewards => totalWeight
	// delRewards   => delWeight

	// delRewards = (totalRewards x delWeight) / totalWeight
	delTotalRewards := sdk.NewCoins()
	for _, totalReward := range totalRewards {
		rwdMulDelWeight := totalReward.Amount.Mul(delWeight)
		delRewards := sdk.NewCoin(totalReward.Denom, rwdMulDelWeight.Quo(totalWeight))
		delTotalRewards = delTotalRewards.Add(delRewards)
	}

	return delTotalRewards
}

func (k Keeper) AcumulateDelRewards(ctx context.Context, del sdk.AccAddress, coins sdk.Coins) error {
	rewards, err := k.GetDelRewards(ctx, del)
	if err != nil {
		return err
	}

	k.SetDelRewards(ctx, del, rewards.Add(coins...))
	return nil
}

// storeDelRewards returns the KVStore of the delegator rewards
// prefix: (DelegatorRewardsKey)
// key: Del addr
// value: sdk coins rewards
func (k Keeper) storeDelRewards(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.DelegatorRewardsKey)
}

// storeDelRewardsByDelegator returns the KVStore of the delegator rewards
// prefix: (DelegatorRewardsKey)
// key: Del addr
// value: sdk coins rewards
func (k Keeper) storeDelRewardsByDelegator(ctx context.Context, del sdk.AccAddress) prefix.Store {
	st := k.storeDelRewards(ctx)
	return prefix.NewStore(st, del)
}

func (k Keeper) GetDelRewards(ctx context.Context, del sdk.AccAddress) (sdk.Coins, error) {
	st := k.storeDelRewards(ctx)
	v := st.Get(del)
	if len(v) == 0 {
		return sdk.NewCoins(), nil
	}

	// todo: use cdc ?
	var coins sdk.Coins
	err := json.Unmarshal(v, &coins)
	if err != nil {
		return nil, err
	}

	return coins, nil
}

func (k Keeper) SetDelRewards(ctx context.Context, del sdk.AccAddress, coins sdk.Coins) {
	st := k.storeDelRewards(ctx)

	bz, err := coins.MarshalJSON()
	if err != nil {
		panic(err)
	}

	st.Set(del, bz)
}

func addDecimals(amt math.Int) math.Int {
	return amt.Mul(decimals_precision)
}

func rmvDecimals(amt math.Int) math.Int {
	return amt.Quo(decimals_precision)
}
