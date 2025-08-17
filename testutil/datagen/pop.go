package datagen

import (
	"github.com/babylonlabs-io/babylon/v4/crypto/bip322"
	"github.com/babylonlabs-io/babylon/v4/crypto/ecdsa"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Bunch of test signing utilites
func PubkeyToP2WPKHAddress(p *btcec.PublicKey, net *chaincfg.Params) (*btcutil.AddressWitnessPubKeyHash, error) {
	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(
		btcutil.Hash160(p.SerializeCompressed()),
		net,
	)

	if err != nil {
		return nil, err
	}

	return witnessAddr, nil
}

func SignWithP2WPKHAddress(
	msg []byte,
	privKey *btcec.PrivateKey,
	net *chaincfg.Params,
) (*btcutil.AddressWitnessPubKeyHash, []byte, error) {
	pubKey := privKey.PubKey()

	witnessAddr, err := PubkeyToP2WPKHAddress(pubKey, net)

	if err != nil {
		return nil, nil, err
	}

	toSpend, err := bip322.GetToSpendTx(msg, witnessAddr)

	if err != nil {
		return nil, nil, err
	}

	toSign := bip322.GetToSignTx(toSpend)

	fetcher := txscript.NewCannedPrevOutputFetcher(
		toSpend.TxOut[0].PkScript,
		toSpend.TxOut[0].Value,
	)

	hashCache := txscript.NewTxSigHashes(toSign, fetcher)

	// always use compressed pubkey
	witness, err := txscript.WitnessSignature(toSign, hashCache, 0,
		toSpend.TxOut[0].Value, toSpend.TxOut[0].PkScript, txscript.SigHashAll, privKey, true)

	if err != nil {
		return nil, nil, err
	}

	serializedWitness, err := bip322.SerializeWitness(witness)

	if err != nil {
		return nil, nil, err
	}

	return witnessAddr, serializedWitness, nil
}

func SignWithP2TrSpendAddress(
	msg []byte,
	privKey *btcec.PrivateKey,
	net *chaincfg.Params,
) (*btcutil.AddressTaproot, []byte, error) {
	pubKey := privKey.PubKey()

	witnessAddr, err := bip322.PubKeyToP2TrSpendAddress(pubKey, net)

	if err != nil {
		return nil, nil, err
	}

	toSpend, err := bip322.GetToSpendTx(msg, witnessAddr)

	if err != nil {
		return nil, nil, err
	}

	toSign := bip322.GetToSignTx(toSpend)

	fetcher := txscript.NewCannedPrevOutputFetcher(
		toSpend.TxOut[0].PkScript,
		toSpend.TxOut[0].Value,
	)

	hashCache := txscript.NewTxSigHashes(toSign, fetcher)

	witness, err := txscript.TaprootWitnessSignature(
		toSign, hashCache, 0, toSpend.TxOut[0].Value, toSpend.TxOut[0].PkScript,
		txscript.SigHashDefault, privKey,
	)

	if err != nil {
		return nil, nil, err
	}

	serializedWitness, err := bip322.SerializeWitness(witness)

	if err != nil {
		return nil, nil, err
	}

	return witnessAddr, serializedWitness, nil
}

type bip322Sign[A btcutil.Address] func(sg []byte,
	privKey *btcec.PrivateKey,
	net *chaincfg.Params) (A, []byte, error)

// NewPoPBTC generates a new proof of possession that sk_BTC and the address are held by the same person
// a proof of possession contains only one signature
// - pop.BtcSig = schnorr_sign(sk_BTC, bbnAddress)
func NewPoPBTC(
	signingContext string,
	addr sdk.AccAddress,
	btcSK *btcec.PrivateKey,
) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_BIP340, // by default, we use BIP-340 encoding for BTC signature
	}

	// generate pop.BtcSig = schnorr_sign(sk_BTC, hash(bbnAddress))
	// NOTE: *schnorr.Sign has to take the hash of the message.
	// So we have to hash the address before signing
	msgToSign := []byte(signingContext)
	msgToSign = append(msgToSign, addr.Bytes()...)
	hash := tmhash.Sum(msgToSign)
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
func NewPoPBTCWithECDSABTCSig(
	signingContext string,
	addr sdk.AccAddress,
	btcSK *btcec.PrivateKey,
) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_ECDSA,
	}

	// generate pop.BtcSig = ecdsa_sign(sk_BTC, pop.BabylonSig)
	// NOTE: ecdsa.Sign has to take the message as string.
	// So we have to convert the address to bech32 string before signing
	addrBech32 := addr.String()
	btcSig := ecdsa.Sign(btcSK, signingContext+addrBech32)
	pop.BtcSig = btcSig

	return &pop, nil
}

func newPoPBTCWithBIP322Sig[A btcutil.Address](
	signingContext string,
	addressToSign sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
	bip322SignFn bip322Sign[A],
) (*bstypes.ProofOfPossessionBTC, error) {
	pop := bstypes.ProofOfPossessionBTC{
		BtcSigType: bstypes.BTCSigType_BIP322,
	}

	bzToSign := bstypes.MsgToSignBIP322(signingContext, addressToSign)
	bip322SigEncoded, err := newBIP322Sig(bzToSign, btcSK, net, bip322SignFn)
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
	signingContext string,
	addr sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
) (*bstypes.ProofOfPossessionBTC, error) {
	return newPoPBTCWithBIP322Sig(signingContext, addr, btcSK, net, SignWithP2WPKHAddress)
}

func NewPoPBTCWithBIP322P2TRBIP86Sig(
	signingContext string,
	addrToSign sdk.AccAddress,
	btcSK *btcec.PrivateKey,
	net *chaincfg.Params,
) (*bstypes.ProofOfPossessionBTC, error) {
	return newPoPBTCWithBIP322Sig(signingContext, addrToSign, btcSK, net, SignWithP2TrSpendAddress)
}
