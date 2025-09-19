package replay

import (
	"crypto/sha256"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type Staker struct {
	*SenderInfo
	r             *rand.Rand
	t             *testing.T
	d             *BabylonAppDriver
	app           *babylonApp.BabylonApp
	BTCPrivateKey *btcec.PrivateKey
}

func (s *Staker) BTCPublicKey() *bbn.BIP340PubKey {
	pk := bbn.NewBIP340PubKeyFromBTCPK(s.BTCPrivateKey.PubKey())
	return pk
}

func (s *Staker) CreateDelegationMessage(
	fpKeys []*bbn.BIP340PubKey,
	stakingTime uint32,
	totalSat int64,
) *bstypes.MsgCreateBTCDelegation {
	params := s.d.GetBTCStakingParams(s.t)

	var covenantPks []*btcec.PublicKey
	for _, pk := range params.CovenantPks {
		covenantPks = append(covenantPks, pk.MustToBTCPK())
	}

	var fpBTCPKs []*btcec.PublicKey
	for _, fpKey := range fpKeys {
		fpBTCPKs = append(fpBTCPKs, fpKey.MustToBTCPK())
	}

	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfo(
		s.r,
		s.t,
		BtcParams,
		s.BTCPrivateKey,
		fpBTCPKs,
		covenantPks,
		params.CovenantQuorum,
		uint16(stakingTime),
		totalSat,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(s.t, err)

	// delegator pre-signs slashing tx
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		s.BTCPrivateKey,
	)
	require.NoError(s.t, err)

	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(s.t, err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(totalSat) - uint64(params.UnbondingFeeSat)

	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		s.r,
		s.t,
		BtcParams,
		s.BTCPrivateKey,
		fpBTCPKs,
		covenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(s.t, err)
	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(s.BTCPrivateKey)
	require.NoError(s.t, err)

	pop, err := datagen.NewPoPBTC(s.Address(), s.BTCPrivateKey)
	require.NoError(s.t, err)

	// Convert []*BIP340PubKey to []BIP340PubKey
	fpBtcPkList := make([]bbn.BIP340PubKey, len(fpKeys))
	for i, pk := range fpKeys {
		fpBtcPkList[i] = *pk
	}

	msg := &bstypes.MsgCreateBTCDelegation{
		StakerAddr:   s.AddressString(),
		Pop:          pop,
		BtcPk:        s.BTCPublicKey(),
		FpBtcPkList:  fpBtcPkList,
		StakingTime:  stakingTime,
		StakingValue: totalSat,
		StakingTx:    serializedStakingTx,
		// We are using nil for so
		StakingTxInclusionProof:       nil,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
	}

	return msg
}

func (s *Staker) CreateBtcExpandMessage(
	fpKeys []*bbn.BIP340PubKey,
	stakingTime uint32,
	totalSat int64,
	prevStakeTxHash string,
	fundingTx *wire.MsgTx,
) *bstypes.MsgBtcStakeExpand {
	params := s.d.GetBTCStakingParams(s.t)

	// Convert fpKeys to btcec.PublicKey
	var fpBTCPKs []*btcec.PublicKey
	for _, fpKey := range fpKeys {
		fpBTCPKs = append(fpBTCPKs, fpKey.MustToBTCPK())
	}

	// Convert covenant keys
	var covenantPks []*btcec.PublicKey
	for _, pk := range params.CovenantPks {
		covenantPks = append(covenantPks, pk.MustToBTCPK())
	}

	// Convert prevStakeTxHash string to OutPoint
	prevHash, err := chainhash.NewHashFromStr(prevStakeTxHash)
	require.NoError(s.t, err)
	prevStakingOutPoint := wire.NewOutPoint(prevHash, datagen.StakingOutIdx)

	// Convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using multiple inputs
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		s.r,
		s.t,
		BtcParams,
		outPoints,
		s.BTCPrivateKey,
		fpBTCPKs,
		covenantPks,
		params.CovenantQuorum,
		uint16(stakingTime),
		totalSat,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(s.t, err)

	// Sign the slashing tx with delegator key
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		s.BTCPrivateKey,
	)
	require.NoError(s.t, err)

	// Serialize the staking tx bytes
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(s.t, err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(totalSat) - uint64(params.UnbondingFeeSat)

	// Generate unbonding slashing info for the stake expand
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		s.r,
		s.t,
		BtcParams,
		s.BTCPrivateKey,
		fpBTCPKs,
		covenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(s.t, err)

	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(s.BTCPrivateKey)
	require.NoError(s.t, err)

	pop, err := datagen.NewPoPBTC(s.Address(), s.BTCPrivateKey)
	require.NoError(s.t, err)

	// Convert []*BIP340PubKey to []BIP340PubKey slice
	fpBtcPkList := make([]bbn.BIP340PubKey, len(fpKeys))
	for i, pk := range fpKeys {
		fpBtcPkList[i] = *pk
	}

	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	require.NoError(s.t, err)

	msg := &bstypes.MsgBtcStakeExpand{
		StakerAddr:                    s.AddressString(),
		Pop:                           pop,
		BtcPk:                         s.BTCPublicKey(),
		FpBtcPkList:                   fpBtcPkList,
		StakingTime:                   stakingTime,
		StakingValue:                  totalSat,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       nil,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
		PreviousStakingTxHash:         prevStakeTxHash,
		FundingTx:                     fundingTxBz,
	}

	return msg
}

func (s *Staker) CreatePreApprovalDelegation(
	fpKeys []*bbn.BIP340PubKey,
	stakingTime uint32,
	totalSat int64,
) *bstypes.MsgCreateBTCDelegation {
	msg := s.CreateDelegationMessage(fpKeys, stakingTime, totalSat)

	s.SendMessage(msg)

	return msg
}

func (s *Staker) CreateBtcStakeExpand(
	fpKeys []*bbn.BIP340PubKey,
	stakingTime uint32,
	totalSat int64,
	prevStkTx *wire.MsgTx,
) *bstypes.MsgBtcStakeExpand {
	stakingOutput := prevStkTx.TxOut[0]

	// Create a fake outPoint for funding
	dummyData := sha256.Sum256([]byte("dummy funding tx"))
	dummyOutPoint := &wire.OutPoint{
		Hash:  chainhash.Hash(dummyData),
		Index: 0,
	}

	// Generate funding tx for stake expansion
	fundingTx := datagen.GenFundingTx(
		s.t,
		s.r,
		s.app.BTCLightClientKeeper.GetBTCNet(),
		dummyOutPoint,
		totalSat,
		stakingOutput,
	)

	msg := s.CreateBtcExpandMessage(fpKeys, stakingTime, totalSat, prevStkTx.TxHash().String(), fundingTx)
	s.SendMessage(msg)
	return msg
}

func (s *Staker) SendMessage(
	msg sdk.Msg,
) {
	DefaultSendTxWithMessagesSuccess(
		s.t,
		s.app,
		s.SenderInfo,
		msg,
	)

	s.IncSeq()
}

// WithdrawBtcStakingRewards withdraws BTC staking rewards for this staker
func (s *Staker) WithdrawBtcStakingRewards() {
	msg := &ictvtypes.MsgWithdrawReward{
		Type:    ictvtypes.BTC_STAKER.String(),
		Address: s.Address().String(),
	}

	DefaultSendTxWithMessagesSuccess(
		s.t,
		s.app,
		s.SenderInfo,
		msg,
	)
	s.IncSeq()
}

func (s *Staker) UnbondDelegation(
	stakingTxHash *chainhash.Hash,
	stakingTx *wire.MsgTx,
	covSender *CovenantSender,
) {
	params := s.d.GetBTCStakingParams(s.t)

	delegation := s.d.GetBTCDelegation(s.t, stakingTxHash.String())
	require.NotNil(s.t, delegation, "delegation should exist")
	infos := parseInfos(s.t, delegation, params)

	unbondingPathSpendInfo, err := infos.StakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(s.t, err)

	stakingOutput := stakingTx.TxOut[delegation.StakingOutputIdx]

	covenantSKs := covSender.CovenantPrivateKeys()
	covenantSigs := datagen.GenerateSignatures(
		s.t,
		covenantSKs,
		infos.UnbondingSlashingInfo.UnbondingTx,
		stakingOutput,
		unbondingPathSpendInfo.RevealedLeaf,
	)

	stakerSig, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
		infos.UnbondingSlashingInfo.UnbondingTx,
		stakingOutput,
		s.BTCPrivateKey,
		unbondingPathSpendInfo.RevealedLeaf,
	)
	require.NoError(s.t, err)

	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(covenantSigs, stakerSig)
	require.NoError(s.t, err)

	unbondingTxMsg := infos.UnbondingSlashingInfo.UnbondingTx
	unbondingTxMsg.TxIn[0].Witness = witness

	blockWithUnbondingTx, _ := s.d.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{unbondingTxMsg})
	require.Len(s.t, blockWithUnbondingTx.Proofs, 2)

	stakingTxBz, err := bbn.SerializeBTCTx(stakingTx)
	require.NoError(s.t, err)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingTxMsg)
	require.NoError(s.t, err)

	msg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.AddressString(),
		StakingTxHash:                 stakingTxHash.String(),
		StakeSpendingTx:               unbondingTxBytes,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithUnbondingTx.Proofs[1]),
		FundingTransactions:           [][]byte{stakingTxBz},
	}

	s.SendMessage(msg)
}
