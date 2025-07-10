package replay

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	"github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"

	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/stretchr/testify/require"
)

type CovenantSender struct {
	*SenderInfo
	r   *rand.Rand
	t   *testing.T
	d   *BabylonAppDriver
	app *babylonApp.BabylonApp
}

type StakingInfos struct {
	FpPKs                 []*btcec.PublicKey
	CovenantPks           []*btcec.PublicKey
	StakerAddr            string
	StakingTxHash         string
	StakingSlashingInfo   *datagen.TestStakingSlashingInfo
	UnbondingSlashingInfo *datagen.TestUnbondingSlashingInfo
}

func parseInfos(
	t *testing.T,
	resp *bstypes.BTCDelegationResponse,
	params *bstypes.Params,
) *StakingInfos {
	stakingTx, _, err := types.NewBTCTxFromHex(resp.StakingTxHex)
	require.NoError(t, err)
	unbondingTx, _, err := types.NewBTCTxFromHex(resp.UndelegationResponse.UnbondingTxHex)
	require.NoError(t, err)
	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(resp.SlashingTxHex)
	require.NoError(t, err)
	unbondingSlashingTx, err := bstypes.NewBTCSlashingTxFromHex(resp.UndelegationResponse.SlashingTxHex)
	require.NoError(t, err)

	fpPKs := make([]*btcec.PublicKey, len(resp.FpBtcPkList))
	for i, pk := range resp.FpBtcPkList {
		fpPKs[i] = pk.MustToBTCPK()
	}

	covenantPks := make([]*btcec.PublicKey, len(params.CovenantPks))

	for i, pk := range params.CovenantPks {
		covenantPks[i] = pk.MustToBTCPK()
	}

	stakingInfo, err := btcstaking.BuildStakingInfo(
		resp.BtcPk.MustToBTCPK(),
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		uint16(resp.StakingTime),
		btcutil.Amount(resp.TotalSat),
		BtcParams,
	)
	require.NoError(t, err)

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		resp.BtcPk.MustToBTCPK(),
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		uint16(resp.UnbondingTime),
		btcutil.Amount(resp.TotalSat-uint64(params.UnbondingFeeSat)),
		BtcParams,
	)
	require.NoError(t, err)

	testStakingInfo := datagen.TestStakingSlashingInfo{
		StakingTx:   stakingTx,
		SlashingTx:  slashingTx,
		StakingInfo: stakingInfo,
	}

	testUnbondingInfo := datagen.TestUnbondingSlashingInfo{
		UnbondingTx:   unbondingTx,
		SlashingTx:    unbondingSlashingTx,
		UnbondingInfo: unbondingInfo,
	}

	stkTxHash := stakingTx.TxHash().String()

	return &StakingInfos{
		FpPKs:                 fpPKs,
		CovenantPks:           covenantPks,
		StakerAddr:            resp.StakerAddr,
		StakingTxHash:         stkTxHash,
		StakingSlashingInfo:   &testStakingInfo,
		UnbondingSlashingInfo: &testUnbondingInfo,
	}
}

func (c *CovenantSender) SendCovenantSignatures() {
	pendingDels := c.d.GetPendingBTCDelegations(c.t)
	params := c.d.GetBTCStakingParams(c.t)

	for _, del := range pendingDels {
		infos := parseInfos(c.t, del, params)

		slashingPkScriptPath, err := infos.StakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
		require.NoError(c.t, err)
		unbondingPkScriptPath, err := infos.StakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
		require.NoError(c.t, err)

		covenantSigs, err := datagen.GenCovenantAdaptorSigs(
			covenantSKs,
			infos.FpPKs,
			infos.StakingSlashingInfo.StakingTx,
			slashingPkScriptPath.GetPkScriptPath(),
			infos.StakingSlashingInfo.SlashingTx,
		)
		require.NoError(c.t, err)

		covUnbondingSlashingSigs, covUnbondingSigs, err := infos.UnbondingSlashingInfo.GenCovenantSigs(
			covenantSKs,
			infos.FpPKs,
			infos.StakingSlashingInfo.StakingTx,
			unbondingPkScriptPath.GetPkScriptPath(),
		)
		require.NoError(c.t, err)

		isStakeExpansion := del.StkExp != nil

		for i := 0; i < len(infos.CovenantPks); i++ {
			msgAddCovenantSig := &bstypes.MsgAddCovenantSigs{
				Signer:                  c.AddressString(),
				Pk:                      covenantSigs[i].CovPk,
				StakingTxHash:           infos.StakingTxHash,
				SlashingTxSigs:          covenantSigs[i].AdaptorSigs,
				UnbondingTxSig:          covUnbondingSigs[i].Sig,
				SlashingUnbondingTxSigs: covUnbondingSlashingSigs[i].AdaptorSigs,
			}

			// if is stake expansion, add StakeExpansionTxSig
			if isStakeExpansion {
				prevDel := c.d.GetBTCDelegation(c.t, del.StkExp.PreviousStakingTxHashHex)
				prevDelInfos := parseInfos(c.t, prevDel, params)
				require.NotNil(c.t, prevDel, "previous delegation should not be nil")
				msgAddCovenantSig.StakeExpansionTxSig = signStakeExpansionTx(c.t, covenantSKs[i], del, infos, prevDelInfos)
			}

			// send each message with different transactions
			DefaultSendTxWithMessagesSuccess(
				c.t,
				c.app,
				c.SenderInfo,
				msgAddCovenantSig,
			)

			c.IncSeq()
		}
	}
}

func signStakeExpansionTx(t *testing.T, covenantSK *btcec.PrivateKey, del *bstypes.BTCDelegationResponse, infos, prevDelInfos *StakingInfos) *types.BIP340Signature {
	fundingTxBz, err := hex.DecodeString(del.StkExp.OtherFundingTxOutHex)
	require.NoError(t, err)
	otherFundingTxOut, err := btcstaking.DeserializeTxOut(fundingTxBz)
	require.NoError(t, err)

	prevDelUnbondPathSpendInfo, err := prevDelInfos.StakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	sig, err := btcstaking.SignTxForFirstScriptSpendWithTwoInputsFromScript(
		infos.StakingSlashingInfo.StakingTx,
		prevDelInfos.StakingSlashingInfo.StakingTx.TxOut[0],
		otherFundingTxOut,
		covenantSK,
		prevDelUnbondPathSpendInfo.GetPkScriptPath(),
	)
	require.NoError(t, err)

	return types.NewBIP340SignatureFromBTCSig(sig)
}
