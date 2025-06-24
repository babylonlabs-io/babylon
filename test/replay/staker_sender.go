package replay

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/types"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/stretchr/testify/require"

	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
	"github.com/btcsuite/btcd/btcec/v2"
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

// CreateDelegationMessage creates all data required to create a delegation and pack
// it into MsgCreateBTCDelegation, message is not sent to the mempool.
// Message does not contain Inclusion proof as produced staking tx is not yet sent
// to the BTC chain.
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

func (s *Staker) CreatePreApprovalDelegation(
	fpKeys []*bbn.BIP340PubKey,
	stakingTime uint32,
	totalSat int64,
) *bstypes.MsgCreateBTCDelegation {
	msg := s.CreateDelegationMessage(fpKeys, stakingTime, totalSat)

	s.SendCreateDelegationMessage(msg)

	return msg
}

func (s *Staker) SendCreateDelegationMessage(
	msg *bstypes.MsgCreateBTCDelegation,
) {
	DefaultSendTxWithMessagesSuccess(
		s.t,
		s.app,
		s.SenderInfo,
		msg,
	)
	s.IncSeq()
}

func StakingTransaction(t *testing.T, msg *bstypes.MsgCreateBTCDelegation) *wire.MsgTx {
	stakingTx, err := types.NewBTCTxFromBytes(msg.StakingTx)
	require.NoError(t, err)
	return stakingTx
}
