package replay

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	"github.com/btcsuite/btcd/btcec/v2"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

type FinalityProvider struct {
	*SenderInfo
	r             *rand.Rand
	t             *testing.T
	d             *BabylonAppDriver
	app           *babylonApp.BabylonApp
	BTCPrivateKey *btcec.PrivateKey
	Description   *stakingtypes.Description
	randListInfo  *datagen.RandListInfo
}

func (f *FinalityProvider) BTCPublicKey() *bbn.BIP340PubKey {
	pk := bbn.NewBIP340PubKeyFromBTCPK(f.BTCPrivateKey.PubKey())
	return pk
}

func (f *FinalityProvider) RegisterFinalityProvider(bsnId string) {
	pop, err := datagen.NewPoPBTC(f.d.FpPopContext(), f.Address(), f.BTCPrivateKey)
	require.NoError(f.t, err)

	msg := &bstypes.MsgCreateFinalityProvider{
		Addr:        f.AddressString(),
		Description: f.Description,
		// Default values
		Commission: bstypes.CommissionRates{
			Rate:          sdkmath.LegacyMustNewDecFromStr("0.05"),
			MaxRate:       sdkmath.LegacyMustNewDecFromStr("0.1"),
			MaxChangeRate: sdkmath.LegacyMustNewDecFromStr("0.05"),
		},
		BsnId: bsnId,
		BtcPk: f.BTCPublicKey(),
		Pop:   pop,
	}

	DefaultSendTxWithMessagesSuccess(
		f.t,
		f.app,
		f.SenderInfo,
		msg,
	)
	// message accepted to the mempool increment sequence number
	f.IncSeq()
}

// WithdrawBtcStakingRewards withdraws BTC staking rewards for this fp
func (f *FinalityProvider) WithdrawBtcStakingRewards() {
	msg := &ictvtypes.MsgWithdrawReward{
		Type:    ictvtypes.FINALITY_PROVIDER.String(),
		Address: f.Address().String(),
	}

	DefaultSendTxWithMessagesSuccess(
		f.t,
		f.app,
		f.SenderInfo,
		msg,
	)
	f.IncSeq()
}

func (f *FinalityProvider) CommitRandomness() {
	randListInfo, msg, err := datagen.GenRandomMsgCommitPubRandList(
		f.r,
		f.BTCPrivateKey,
		f.d.FpRandCommitContext(),
		1,
		10000,
	)
	require.NoError(f.t, err)

	msg.Signer = f.AddressString()

	DefaultSendTxWithMessagesSuccess(
		f.t,
		f.app,
		f.SenderInfo,
		msg,
	)

	// TODO: for now only one commmitment is supported
	f.randListInfo = randListInfo

	f.IncSeq()
}

func (f *FinalityProvider) CastVote(height uint64) {
	indexedBlock := f.d.GetIndexedBlock(height)
	f.CastVoteForHash(height, indexedBlock.AppHash)
}

// CastVoteForHash useful to cast bad vote
func (f *FinalityProvider) CastVoteForHash(height uint64, blkAppHash []byte) {
	msg, err := datagen.NewMsgAddFinalitySig(
		f.AddressString(),
		f.BTCPrivateKey,
		f.d.FpFinVoteContext(),
		1,
		height,
		f.randListInfo,
		blkAppHash,
	)
	require.NoError(f.t, err)

	DefaultSendTxWithMessagesSuccess(
		f.t,
		f.app,
		f.SenderInfo,
		msg,
	)

	f.IncSeq()
}

func (f *FinalityProvider) SendSelectiveSlashingEvidence() {
	msg := &bstypes.MsgSelectiveSlashingEvidence{
		Signer:           f.AddressString(),
		RecoveredFpBtcSk: f.BTCPrivateKey.Serialize(),
	}

	DefaultSendTxWithMessagesSuccess(
		f.t,
		f.app,
		f.SenderInfo,
		msg,
	)

	f.IncSeq()
}
