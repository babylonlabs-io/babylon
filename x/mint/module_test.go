package mint_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/mint/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

func TestItCreatesModuleAccountOnInitBlock(t *testing.T) {
	h := helper.NewHelper(t)
	app, ctx := h.App, h.Ctx

	acc := app.AccountKeeper.GetAccount(ctx, authtypes.NewModuleAddress(types.ModuleName))
	require.NotNil(t, acc)
}
