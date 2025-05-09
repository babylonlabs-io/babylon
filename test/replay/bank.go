package replay

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/test-go/testify/require"
)

func (d *BabylonAppDriver) BankBalances(addrs ...sdk.AccAddress) map[string]sdk.Coins {
	bankK := d.App.BankKeeper

	result := make(map[string]sdk.Coins, len(addrs))
	for _, addr := range addrs {
		balances := bankK.GetAllBalances(d.Ctx(), addr)
		result[addr.String()] = balances
	}

	return result
}

func (d *BabylonAppDriver) BankBalance(denom string, addrs ...sdk.AccAddress) map[string]sdk.Coin {
	bankK := d.App.BankKeeper

	result := make(map[string]sdk.Coin, len(addrs))
	for _, addr := range addrs {
		balance := bankK.GetBalance(d.Ctx(), addr, denom)
		result[addr.String()] = balance
	}

	return result
}

func (d *BabylonAppDriver) BankBalanceBond(addrs ...sdk.AccAddress) map[string]sdk.Coin {
	bondDenom, err := d.App.StakingKeeper.BondDenom(d.Ctx())
	require.NoError(d.t, err)
	return d.BankBalance(bondDenom, addrs...)
}
