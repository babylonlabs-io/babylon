package keeper_test

import (
	"context"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/x/btcdistribution/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.StakingKeeper = MockStk{}

type MockBtcStk struct {
	btcDels map[string]math.Int
}

func NewMockBtcStk(btcDels map[string]math.Int) types.BTCStakingKeeper {
	return MockBtcStk{
		btcDels: btcDels,
	}
}

// IterateBTCDelegators implements types.BTCStakingKeeper.
func (m MockBtcStk) IterateBTCDelegators(ctx context.Context, i func(delegator sdk.AccAddress, totalSatoshiStaked math.Int) error) error {
	for delAddrStr, amtStk := range m.btcDels {
		delAddr := sdk.MustAccAddressFromBech32(delAddrStr)
		err := i(delAddr, amtStk)
		if err != nil {
			return err
		}
	}
	return nil
}

// TotalSatoshiStaked implements types.BTCStakingKeeper.
func (m MockBtcStk) TotalSatoshiStaked(ctx context.Context) (math.Int, error) {
	t := math.NewInt(0)
	for _, v := range m.btcDels {
		t = t.Add(v)
	}
	return t, nil
}

type MockStk struct {
	btcDels map[string]math.Int
}

func NewMockStk(btcDels map[string]math.Int) types.StakingKeeper {
	return MockStk{
		btcDels: btcDels,
	}
}

// GetDelegatorBonded implements types.StakingKeeper.
func (m MockStk) GetDelegatorBonded(ctx context.Context, delegator sdk.AccAddress) (math.Int, error) {
	return m.btcDels[delegator.String()], nil
}

// TotalBondedTokens implements types.StakingKeeper.
func (m MockStk) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	t := math.NewInt(0)
	for _, v := range m.btcDels {
		t = t.Add(v)
	}
	return t, nil
}
