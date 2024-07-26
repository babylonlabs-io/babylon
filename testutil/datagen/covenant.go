package datagen

import (
	"github.com/babylonlabs-io/babylon/btcstaking"
	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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
