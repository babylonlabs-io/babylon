package datagen

import (
	"errors"

	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/wire"
)

func GenCovenantAdaptorSigs(
	covenantSKs []*btcec.PrivateKey,
	fpPKs []*btcec.PublicKey,
	fundingTx *wire.MsgTx,
	pkScriptPath []byte,
	slashingTx *bstypes.BTCSlashingTx,
) ([]*bstypes.CovenantAdaptorSignatures, error) {
	covenantSigs := []*bstypes.CovenantAdaptorSignatures{}
	for _, covenantSK := range covenantSKs {
		covMemberSigs := &bstypes.CovenantAdaptorSignatures{
			CovPk:       bbn.NewBIP340PubKeyFromBTCPK(covenantSK.PubKey()),
			AdaptorSigs: [][]byte{},
		}
		for _, fpPK := range fpPKs {
			encKey, err := asig.NewEncryptionKeyFromBTCPK(fpPK)
			if err != nil {
				return nil, err
			}
			covenantSig, err := slashingTx.EncSign(fundingTx, 0, pkScriptPath, covenantSK, encKey)
			if err != nil {
				return nil, err
			}
			covMemberSigs.AdaptorSigs = append(covMemberSigs.AdaptorSigs, covenantSig.MustMarshal())
		}
		covenantSigs = append(covenantSigs, covMemberSigs)
	}

	return covenantSigs, nil
}

func GenCovenantUnbondingSigs(covenantSKs []*btcec.PrivateKey, stakingTx *wire.MsgTx, stakingOutIdx uint32, unbondingPkScriptPath []byte, unbondingTx *wire.MsgTx) ([]*schnorr.Signature, error) {
	sigs := []*schnorr.Signature{}
	for i := range covenantSKs {
		sig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
			unbondingTx,
			stakingTx,
			stakingOutIdx,
			unbondingPkScriptPath,
			covenantSKs[i],
		)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func GenCovenantStakeExpSig(covenantSKs []*btcec.PrivateKey, del *bstypes.BTCDelegation, prevDelStakingInfo *btcstaking.StakingInfo) ([]*bbn.BIP340Signature, error) {
	if del.StkExp == nil {
		return nil, errors.New("cannot generate stake expansion sigs for non-stake-expansion delegation")
	}
	otherFundingTxOut, err := del.StkExp.FundingTxOut()
	if err != nil {
		return nil, err
	}

	prevDelUnbondPathSpendInfo, err := prevDelStakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return nil, err
	}
	sigs := []*bbn.BIP340Signature{}

	for i := range covenantSKs {
		sig, err := btcstaking.SignTxForFirstScriptSpendWithTwoInputsFromScript(
			del.MustGetStakingTx(),
			prevDelStakingInfo.StakingOutput,
			otherFundingTxOut,
			covenantSKs[i],
			prevDelUnbondPathSpendInfo.GetPkScriptPath(),
		)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, bbn.NewBIP340SignatureFromBTCSig(sig))
	}

	return sigs, nil
}
