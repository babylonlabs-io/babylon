package types

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/babylonlabs-io/babylon/v3/crypto/bip322"
	"github.com/babylonlabs-io/babylon/v3/crypto/ecdsa"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type checkStakerKey func(stakerKey *bbn.BIP340PubKey) error

func NewPoPBTCFromHex(popHex string) (*ProofOfPossessionBTC, error) {
	popBytes, err := hex.DecodeString(popHex)
	if err != nil {
		return nil, err
	}
	var pop ProofOfPossessionBTC
	if err := pop.Unmarshal(popBytes); err != nil {
		return nil, err
	}
	return &pop, nil
}

func (pop *ProofOfPossessionBTC) ToHexStr() (string, error) {
	popBytes, err := pop.Marshal()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(popBytes), nil
}

// Verify that the BTC private key corresponding to the bip340PK signed the staker address
func (pop *ProofOfPossessionBTC) Verify(
	signingContext string,
	address sdk.AccAddress,
	bip340PK *bbn.BIP340PubKey,
	net *chaincfg.Params,
) error {
	stakerBech32Addr := address.String()
	switch pop.BtcSigType {
	case BTCSigType_BIP340:
		return pop.VerifyBIP340(signingContext, address, bip340PK)
	case BTCSigType_BIP322:
		return pop.VerifyBIP322(signingContext, address, bip340PK, net)
	case BTCSigType_ECDSA:
		return pop.VerifyECDSA(signingContext, stakerBech32Addr, bip340PK)
	default:
		return fmt.Errorf("invalid BTC signature type")
	}
}

// VerifyBIP340 if the BTC signature has signed the hash by the pair of bip340PK.
func VerifyBIP340(sigType BTCSigType, btcSigRaw []byte, bip340PK *bbn.BIP340PubKey, msg []byte) error {
	if sigType != BTCSigType_BIP340 {
		return fmt.Errorf("the Bitcoin signature in this proof of possession is not using BIP-340 encoding")
	}

	bip340Sig, err := bbn.NewBIP340Signature(btcSigRaw)
	if err != nil {
		return err
	}
	btcSig, err := bip340Sig.ToBTCSig()
	if err != nil {
		return err
	}
	btcPK, err := bip340PK.ToBTCPK()
	if err != nil {
		return err
	}

	// NOTE: btcSig.Verify has to take hash of the message.
	// So we have to hash babylonSig before verifying the signature
	hash := tmhash.Sum(msg)
	if !btcSig.Verify(hash, btcPK) {
		return fmt.Errorf("failed to verify pop.BtcSig")
	}

	return nil
}

// VerifyBIP340 verifies the validity of PoP where Bitcoin signature is in BIP-340
// 1. verify(sig=sig_btc, pubkey=pk_btc, msg=staker_addr)?
func (pop *ProofOfPossessionBTC) VerifyBIP340(contextString string, stakerAddr sdk.AccAddress, bip340PK *bbn.BIP340PubKey) error {
	msgToSign := []byte(contextString)
	msgToSign = append(msgToSign, stakerAddr.Bytes()...)
	return VerifyBIP340(pop.BtcSigType, pop.BtcSig, bip340PK, msgToSign)
}

// isSupportedAddressAndWitness checks whether provided address and witness are
// valid for proof of possession verification.
// Currently the only supported options are:
// 1. p2wpkh address which should only 2 elements in witness: signature and public key
// 2. p2tr address which should only 1 element in witness: signature i.e p2tr key spend
// If validation succeeds, it returns a function which can be used to check whether
// bip340PK corresponds to verified address.
func isSupportedAddressAndWitness(
	address btcutil.Address,
	witness wire.TxWitness,
	net *chaincfg.Params) (checkStakerKey, error) {
	script, err := txscript.PayToAddrScript(address)

	if err != nil {
		return nil, err
	}

	// pay to taproot key spending path have only signature in witness
	if txscript.IsPayToTaproot(script) && len(witness) == 1 {
		return func(stakerKey *bbn.BIP340PubKey) error {
			btcKey, err := stakerKey.ToBTCPK()

			if err != nil {
				return err
			}

			keyAddress, err := bip322.PubKeyToP2TrSpendAddress(btcKey, net)

			if err != nil {
				return err
			}

			if !bytes.Equal(keyAddress.ScriptAddress(), address.ScriptAddress()) {
				return fmt.Errorf("bip322Sig.Address does not correspond to bip340PK")
			}

			return nil
		}, nil
	}

	// pay to witness key hash have signature and public key in witness
	if txscript.IsPayToWitnessPubKeyHash(script) && len(witness) == 2 {
		return func(stakerKey *bbn.BIP340PubKey) error {
			keyFromWitness, err := btcec.ParsePubKey(witness[1])

			if err != nil {
				return err
			}

			keyFromWitnessBytess := schnorr.SerializePubKey(keyFromWitness)

			stakerKeyEncoded, err := stakerKey.Marshal()

			if err != nil {
				return err
			}

			if !bytes.Equal(keyFromWitnessBytess, stakerKeyEncoded) {
				return fmt.Errorf("bip322Sig.Address does not correspond to bip340PK")
			}

			return nil
		}, nil
	}

	return nil, fmt.Errorf("unsupported bip322 address type. Only supported options are p2wpkh and p2tr bip86 key spending path")
}

// VerifyBIP322SigPop verifies bip322 `signature` over `msg` and also checks whether
// `address` corresponds to `pubKeyNoCoord` in the given network.
// It supports only two type of addresses:
// 1. p2wpkh address
// 2. p2tr address which is defined in bip88
// Parameters:
// - msg: message which was signed
// - address: address which was used to sign the message
// - signature: bip322 signature over the message
// - pubKeyNoCoord: public key in 32 bytes format which was used to derive address
func VerifyBIP322SigPop(
	msg []byte,
	address string,
	signature []byte,
	pubKeyNoCoord []byte,
	net *chaincfg.Params,
) error {
	if len(msg) == 0 || len(address) == 0 || len(signature) == 0 || len(pubKeyNoCoord) == 0 {
		return fmt.Errorf("cannot verify bip322 signature. One of the required parameters is empty")
	}

	witness, err := bip322.SimpleSigToWitness(signature)
	if err != nil {
		return err
	}

	btcAddress, err := btcutil.DecodeAddress(address, net)
	if err != nil {
		return err
	}

	// we check whether address and witness are valid for proof of possession verification
	// before verifying bip322 signature. This is require to avoid cases in which
	// we receive some long running btc script to execute (like taproot script with 100 signatures)
	// for proof of possession, we only support two types of cases:
	// 1. address is p2wpkh address
	// 2. address is p2tr address and we are dealing with bip86 (https://github.com/bitcoin/bips/blob/master/bip-0086.mediawiki)
	// key spending path.
	// In those two cases we are able to link bip340PK public key to the btc address
	// used in bip322 signature verification.
	stakerKeyMatchesBtcAddressFn, err := isSupportedAddressAndWitness(btcAddress, witness, net)
	if err != nil {
		return err
	}

	if err := bip322.Verify(msg, witness, btcAddress, net); err != nil {
		return err
	}

	key, err := bbn.NewBIP340PubKey(pubKeyNoCoord)
	if err != nil {
		return err
	}

	// rule 3: verify bip322Sig.Address corresponds to bip340PK
	if err := stakerKeyMatchesBtcAddressFn(key); err != nil {
		return err
	}

	return nil
}

// VerifyBIP322 verifies the validity of PoP where Bitcoin signature is in BIP-322
// after decoding pop.BtcSig to bip322Sig which contains sig and address,
// verify whether bip322 pop signature where msg=signedMsg
func VerifyBIP322(sigType BTCSigType, btcSigRaw []byte, bip340PK *bbn.BIP340PubKey, signedMsg []byte, net *chaincfg.Params) error {
	if sigType != BTCSigType_BIP322 {
		return fmt.Errorf("the Bitcoin signature in this proof of possession is not using BIP-322 encoding")
	}
	// unmarshal pop.BtcSig to bip322Sig
	var bip322Sig BIP322Sig
	if err := bip322Sig.Unmarshal(btcSigRaw); err != nil {
		return err
	}

	btcKeyBytes, err := bip340PK.Marshal()
	if err != nil {
		return err
	}

	// Verify Bip322 proof of possession signature
	if err := VerifyBIP322SigPop(
		signedMsg,
		bip322Sig.Address,
		bip322Sig.Sig,
		btcKeyBytes,
		net,
	); err != nil {
		return err
	}

	return nil
}

// VerifyBIP322 verifies the validity of PoP where Bitcoin signature is in BIP-322
// after decoding pop.BtcSig to bip322Sig which contains sig and address,
// 1. verify whether bip322 pop signature where msg=pop.BabylonSig
// 2. verify(sig=pop.BabylonSig, pubkey=babylonPK, msg=bip340PK)?
func (pop *ProofOfPossessionBTC) VerifyBIP322(contextString string, addr sdk.AccAddress, bip340PK *bbn.BIP340PubKey, net *chaincfg.Params) error {
	msgToSign := MsgToSignBIP322(contextString, addr)
	if err := VerifyBIP322(pop.BtcSigType, pop.BtcSig, bip340PK, msgToSign, net); err != nil {
		return fmt.Errorf("failed to verify possession of babylon sig by the BTC key: %w", err)
	}
	return nil
}

// MsgToSignBIP322 gets the address as bech32 string and just parses it to bytes
// This is necessary due to wallet extensions only allow to sign string messages and
// convert those to bytes before signing.
func MsgToSignBIP322(contextString string, addr sdk.AccAddress) []byte {
	bech32AddrStr := addr.String()
	return []byte(contextString + bech32AddrStr)
}

// VerifyECDSA verifies the validity of PoP where Bitcoin signature is in ECDSA encoding
// 1. verify(sig=sig_btc, pubkey=pk_btc, msg=msg)?
func VerifyECDSA(sigType BTCSigType, btcSigRaw []byte, bip340PK *bbn.BIP340PubKey, signingContext string, msg string) error {
	if sigType != BTCSigType_ECDSA {
		return fmt.Errorf("the Bitcoin signature in this proof of possession is not using ECDSA encoding")
	}

	// rule 1: verify(sig=sig_btc, pubkey=pk_btc, msg=msg)?
	btcPK, err := bip340PK.ToBTCPK()
	if err != nil {
		return err
	}

	// we ignore ignore is compressed flag as we only care about comparing X coordinates
	recoveredPK, _, err := ecdsa.RecoverPublicKey(signingContext+msg, btcSigRaw)
	if err != nil {
		return fmt.Errorf("failed to recover btc public key when verifying ECDSA PoP: %w", err)
	}

	btcPKBytes := schnorr.SerializePubKey(btcPK)

	recoveredPKBytes := schnorr.SerializePubKey(recoveredPK)

	// We only compare X coordinates, as both stakers and fp providers
	// are identified by 32byte public key compliant with BIP-340 spec that
	// assumes Y coordinate is even.
	// This should be safe to do, as except of the ECDSA pop verification, both
	// both parties will also sign other data with BIP-340 schnorr signature for
	// which 32bytes key is enough.
	if !bytes.Equal(btcPKBytes, recoveredPKBytes) {
		return fmt.Errorf("the recovered PK does not match the given PK")
	}

	return nil
}

// VerifyECDSA verifies the validity of PoP where Bitcoin signature is in ECDSA encoding
// 1. verify(sig=sig_btc, pubkey=pk_btc, msg=addr)?
func (pop *ProofOfPossessionBTC) VerifyECDSA(signingContext string, msg string, bip340PK *bbn.BIP340PubKey) error {
	return VerifyECDSA(pop.BtcSigType, pop.BtcSig, bip340PK, signingContext, msg)
}

// ValidateBasic checks if there is a BTC Signature.
func (pop *ProofOfPossessionBTC) ValidateBasic() error {
	if pop.BtcSig == nil {
		return fmt.Errorf("empty BTC signature")
	}

	switch pop.BtcSigType {
	case BTCSigType_BIP340:
		_, err := bbn.NewBIP340Signature(pop.BtcSig)
		if err != nil {
			return fmt.Errorf("invalid BTC BIP340 signature: %w", err)
		}
		return nil
	case BTCSigType_BIP322:
		var bip322Sig BIP322Sig
		if err := bip322Sig.Unmarshal(pop.BtcSig); err != nil {
			return fmt.Errorf("invalid BTC BIP322 signature: %w", err)
		}
		return nil
	case BTCSigType_ECDSA:
		if len(pop.BtcSig) != 65 { // size of compact signature
			return fmt.Errorf("invalid BTC ECDSA signature size")
		}
		return nil
	default:
		return fmt.Errorf("invalid BTC signature type")
	}
}
