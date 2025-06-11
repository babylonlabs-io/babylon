package replay

import (
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1types "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
)

func (d *BabylonAppDriver) NewGovProp(msgs ...sdk.Msg) sdk.Msg {
	p := d.GovParams()

	msg, err := govv1types.NewMsgSubmitProposal(
		msgs,
		sdk.NewCoins(p.ExpeditedMinDeposit...),
		d.AddressString(),
		"",
		datagen.GenRandomHexStr(d.r, 100),
		datagen.GenRandomHexStr(d.r, 1000),
		true,
	)
	require.NoError(d.t, err)

	return msg
}

func (d *BabylonAppDriver) GovVote(propId uint64) {
	msg := govv1types.NewMsgVote(d.Address(), propId, govv1types.OptionYes, "")

	d.SendTxWithMsgsFromDriverAccount(d.t, msg)
}

func (d *BabylonAppDriver) GovParams() govv1types.Params {
	p, err := d.GovQuerySvr().Params(d.Ctx(), &govv1types.QueryParamsRequest{})
	require.NoError(d.t, err)

	return *p.Params
}
