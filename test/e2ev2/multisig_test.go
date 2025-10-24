package e2e2

import (
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func Test2of3MultisigBtcDel(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTestManager(t)
	cfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	cfg.NodeCount = 2
	tm.Chains[tmanager.CHAIN_ID_BABYLON] = tmanager.NewChain(tm, cfg)
	tm.Start()

	tm.ChainsWaitUntilHeight(3)

	bbns := tm.ChainNodes()
	bbn1 := bbns[0]
	bbn2 := bbns[1]
	bbn1.DefaultWallet().VerifySentTx = true
	bbn2.DefaultWallet().VerifySentTx = true

	// create bbn1 as a fp
	r := rand.New(rand.NewSource(time.Now().Unix()))
	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenCustomFinalityProvider(r, fpSK, bbn1.DefaultWallet().Address)
	require.NoError(t, err)
	bbn1.CreateFinalityProvider(bbn1.DefaultWallet().KeyName, fp)
	bbn1.WaitForNextBlock()

	fpResp := bbn1.QueryFinalityProvider(fp.BtcPk.MarshalHex())
	require.NotNil(t, fpResp)

	// 2-of-3 multisig delegation from bbn2 to fp (bbn1)
	stkSKs, _, err := datagen.GenRandomBTCKeyPairs(r, 3)
	require.NoError(t, err)

	msg, stakingInfo := buildMultisigDelegationMsg(
		t, r, bbn2,
		bbn2.DefaultWallet(),
		stkSKs,
		2,
		fpSK.PubKey(),
		int64(2*10e8),
		1000,
	)

	bbn2.CreateBTCDelegation(bbn2.DefaultWallet().KeyName, msg)
	bbn2.WaitForNextBlock()

	// query and verify delegation
	del := bbn2.QueryBTCDelegation(stakingInfo.StakingTx.TxHash().String())
	require.NotNil(t, del)
	require.Equal(t, "PENDING", del.StatusDesc)
	require.NotNil(t, del.MultisigInfo)
	require.Equal(t, uint32(2), del.MultisigInfo.StakerQuorum)
	require.Len(t, del.MultisigInfo.StakerBtcPkList, 2)
}

func buildMultisigDelegationMsg(
	t *testing.T,
	r *rand.Rand,
	node *tmanager.Node,
	wallet *tmanager.WalletSender,
	stakerSKs []*btcec.PrivateKey,
	stakerQuorum uint32,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
) (*bstypes.MsgCreateBTCDelegation, *datagen.TestStakingSlashingInfo) {
	params := node.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.Params.CovenantPks)
	require.NoError(t, err)

	// generate staking + slashing info
	stakingInfo := datagen.GenMultisigBTCStakingSlashingInfo(
		r, t, net,
		stakerSKs,
		stakerQuorum,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.Params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.Params.SlashingPkScript,
		params.Params.SlashingRate,
		uint16(params.Params.UnbondingTimeBlocks),
	)

	// generate unbonding info
	unbondingValue := stakingValue - params.Params.UnbondingFeeSat
	stkTxHash := stakingInfo.StakingTx.TxHash()

	unbondingInfo := datagen.GenMultisigBTCUnbondingSlashingInfo(
		r, t, net,
		stakerSKs,
		stakerQuorum,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.Params.CovenantQuorum,
		&wire.OutPoint{Hash: stkTxHash, Index: 0},
		uint16(params.Params.UnbondingTimeBlocks),
		unbondingValue,
		params.Params.SlashingPkScript,
		params.Params.SlashingRate,
		uint16(params.Params.UnbondingTimeBlocks),
	)

	// sign slashing tx with primary staker (first one)
	slashingSpendInfo, err := stakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx, 0,
		slashingSpendInfo.GetPkScriptPath(),
		stakerSKs[0],
	)
	require.NoError(t, err)

	// generate extra staker signatures (for remaining stakers)
	var extraSlashingSigs []*bstypes.SignatureInfo
	for _, sk := range stakerSKs[1:] {
		sig, err := stakingInfo.SlashingTx.Sign(
			stakingInfo.StakingTx, 0,
			slashingSpendInfo.GetPkScriptPath(),
			sk,
		)
		require.NoError(t, err)

		extraSlashingSigs = append(extraSlashingSigs, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
			Sig: sig,
		})
	}

	// sign unbonding slashing tx with primary staker
	delUnbondingSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSKs[0])
	require.NoError(t, err)

	// generate extra unbonding signatures
	var extraUnbondingSigs []*bstypes.SignatureInfo
	for _, sk := range stakerSKs[1:] {
		sig, err := unbondingInfo.GenDelSlashingTxSig(sk)
		require.NoError(t, err)

		extraUnbondingSigs = append(extraUnbondingSigs, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
			Sig: sig,
		})
	}

	// generate PoP for primary staker
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSKs[0])
	require.NoError(t, err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(t, err)

	// build extra staker PK list (all stakers except the first one)
	extraStakerPKs := make([]bbn.BIP340PubKey, len(stakerSKs)-1)
	for i, sk := range stakerSKs[1:] {
		extraStakerPKs[i] = *bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey())
	}

	return &bstypes.MsgCreateBTCDelegation{
		StakerAddr:                    wallet.Address.String(),
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSKs[0].PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingTime:                 params.Params.UnbondingTimeBlocks,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
		MultisigInfo: &bstypes.AdditionalStakerInfo{
			StakerBtcPkList:                extraStakerPKs,
			StakerQuorum:                   stakerQuorum,
			DelegatorSlashingSigs:          extraSlashingSigs,
			DelegatorUnbondingSlashingSigs: extraUnbondingSigs,
		},
	}, stakingInfo
}
