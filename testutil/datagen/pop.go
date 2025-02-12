package datagen

import (
	"encoding/hex"

	"github.com/babylonlabs-io/babylon/crypto/bip322"
	"github.com/babylonlabs-io/babylon/crypto/ecdsa"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type bip322Sign[A btcutil.Address] func(sg []byte,
	privKey *btcec.PrivateKey,
	net *chaincfg.Params) (A, []byte, error)

// NewPoPBTC generates a new proof of possession that sk_BTC and the address are held by the same person
// a proof of possession contains only one signature
// - pop.BtcSig = schnorr_sign(sk_BTC, bbnAddress)
func NewPoPBTC(addr sdk.AccAddress, btcSK *btcec.PrivateKey) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_BIP340, // by default, we use BIP-340 encoding for BTC signature
	}

	// generate pop.BtcSig = schnorr_sign(sk_BTC, hash(bbnAddress))
	// NOTE: *schnorr.Sign has to take the hash of the message.
	// So we have to hash the address before signing
	hash := tmhash.Sum(addr.Bytes())
	btcSig, err := schnorr.Sign(btcSK, hash)
	if err != nil {
		return nil, err
	}
	bip340Sig := bbn.NewBIP340SignatureFromBTCSig(btcSig)
	pop.BtcSig = bip340Sig.MustMarshal()

	return &pop, nil
}

// NewPoPWithECDSABTCSig generates a new proof of possession where Bitcoin signature is in ECDSA format
// a proof of possession contains two signatures:
// - pop.BtcSig = ecdsa_sign(sk_BTC, addr)
func NewPoPBTCWithECDSABTCSig(addr sdk.AccAddress, btcSK *btcec.PrivateKey) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_ECDSA,
	}

	// generate pop.BtcSig = ecdsa_sign(sk_BTC, pop.BabylonSig)
	// NOTE: ecdsa.Sign has to take the message as string.
	// So we have to hex addr before signing
	addrHex := hex.EncodeToString(addr.Bytes())
	btcSig := ecdsa.Sign(btcSK, addrHex)
	pop.BtcSig = btcSig

	return &pop, nil
}

func newPoPBTCWithBIP322Sig[A btcutil.Address](
	addressToSign sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
	bip322SignFn bip322Sign[A],
) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_BIP322,
	}

	bip322SigEncoded, err := newBIP322Sig(tmhash.Sum(addressToSign.Bytes()), btcSK, net, bip322SignFn)
	if err != nil {
		return nil, err
	}
	pop.BtcSig = bip322SigEncoded

	return &pop, nil
}

func newBIP322Sig[A btcutil.Address](
	msgToSign []byte,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
	bip322SignFn bip322Sign[A],
) ([]byte, error) {
	address, witnessSignture, err := bip322SignFn(
		msgToSign,
		btcSK,
		net,
	)
	if err != nil {
		return nil, err
	}

	bip322Sig := bstypes.BIP322Sig{
		Address: address.EncodeAddress(),
		Sig:     witnessSignture,
	}

	return bip322Sig.Marshal()
}

// NewPoPBTCWithBIP322P2WPKHSig creates a proof of possession of type BIP322
// that signs the address with the BTC secret key.
func NewPoPBTCWithBIP322P2WPKHSig(
	addr sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
) (*bstypes.ProofOfPossessionBTC, error) {
	return newPoPBTCWithBIP322Sig(addr, btcSK, net, bip322.SignWithP2WPKHAddress)
}

func NewPoPBTCWithBIP322P2TRBIP86Sig(
	addrToSign sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
) (*bstypes.ProofOfPossessionBTC, error) {
	return newPoPBTCWithBIP322Sig(addrToSign, btcSK, net, bip322.SignWithP2TrSpendAddress)
}
