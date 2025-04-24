package replay

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	babylonApp "github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

func (f *FinalityProvider) RegisterFinalityProvider() {
	pop, err := datagen.NewPoPBTC(f.Address(), f.BTCPrivateKey)
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
		ConsumerId: "",
		BtcPk:      f.BTCPublicKey(),
		Pop:        pop,
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

func (f *FinalityProvider) CommitRandomness() {
	randListInfo, msg, err := datagen.GenRandomMsgCommitPubRandList(
		f.r,
		f.BTCPrivateKey,
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

	msg, err := datagen.NewMsgAddFinalitySig(
		f.AddressString(),
		f.BTCPrivateKey,
		1,
		height,
		f.randListInfo,
		indexedBlock.AppHash,
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
