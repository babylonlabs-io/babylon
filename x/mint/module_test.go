package mint_test

import (
	"testing"

<<<<<<< HEAD
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/helper"
	"github.com/babylonlabs-io/babylon/v3/x/mint/types"
=======
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/helper"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
>>>>>>> dfbd055 (chore:  e2e refactory (#1552))
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

func TestItCreatesModuleAccountOnInitBlock(t *testing.T) {
	h := helper.NewHelper(t)
	app, ctx := h.App, h.Ctx

	acc := app.AccountKeeper.GetAccount(ctx, authtypes.NewModuleAddress(types.ModuleName))
	require.NotNil(t, acc)

	feeColl := app.AccountKeeper.GetAccount(ctx, appparams.AccFeeCollector)
	require.Equal(t, "bbn17xpfvakm2amg962yls6f84z3kell8c5l88j35y", feeColl.GetAddress().String())
	require.Equal(t, "bbn1pxp3laljasxl67j0k4lvv9hy8yr9043teh7zry", appparams.AccBbnComissionCollectorBsn.String())
	require.Equal(t, "bbn1hfny2zhlc328ksxjsv3qrrldcgqw3684yu5vsh", authtypes.NewModuleAddress(ictvtypes.ModuleName).String())
	require.Equal(t, "bbn13837feaxn8t0zvwcjwhw7lhpgdcx4s36eqteah", authtypes.NewModuleAddress(btcstktypes.ModuleName).String())
}
