package e2e2

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// BuildSingleSigDelegationMsg constructs a original single-sig BTC delegation message
func BuildSingleSigDelegationMsg(
	t *testing.T,
	r *rand.Rand,
	node *tmanager.Node,
	wallet *tmanager.WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
) (*bstypes.MsgCreateBTCDelegation, *datagen.TestStakingSlashingInfo) {
	params := node.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(t, err)

	// generate staking + slashing info
	stakingInfo := datagen.GenBTCStakingSlashingInfo(
		r, t, net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// generate unbonding info
	unbondingValue := stakingValue - params.UnbondingFeeSat
	stkTxHash := stakingInfo.StakingTx.TxHash()

	unbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r, t, net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		&wire.OutPoint{Hash: stkTxHash, Index: 0},
		uint16(params.UnbondingTimeBlocks),
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// sign slashing tx
	slashingSpendInfo, err := stakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx, 0,
		slashingSpendInfo.GetPkScriptPath(),
		stakerSK,
	)
	require.NoError(t, err)

	// sign unbonding slashing tx
	delUnbondingSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSK)
	require.NoError(t, err)

	// generate PoP
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSK)
	require.NoError(t, err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(t, err)

	return &bstypes.MsgCreateBTCDelegation{
		StakerAddr:                    wallet.Address.String(),
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSK.PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
	}, stakingInfo
}
