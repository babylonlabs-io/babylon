package bip322_test

import (
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/crypto/bip322"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"
)

var (
	net                = &chaincfg.TestNet3Params
	emptyBytes         = []byte("")
	helloWorldBytes    = []byte("Hello World")
	testAddr           = "bc1q9vza2e8x573nczrlzms0wvx3gsqjx7vavgkx0l"
	testAddrDecoded, _ = btcutil.DecodeAddress(testAddr, net)
)

// test vectors at https://github.com/bitcoin/bips/blob/master/bip-0322.mediawiki#message-hashing
func TestBIP322_MsgHash(t *testing.T) {
	msgHash := bip322.GetBIP340TaggedHash(emptyBytes)
	msgHashHex := hex.EncodeToString(msgHash[:])
	require.Equal(t, msgHashHex, "c90c269c4f8fcbe6880f72a721ddfbf1914268a794cbb21cfafee13770ae19f1")

	msgHash = bip322.GetBIP340TaggedHash(helloWorldBytes)
	msgHashHex = hex.EncodeToString(msgHash[:])
	require.Equal(t, msgHashHex, "f0eb03b1a75ac6d9847f55c624a99169b5dccba2a31f5b23bea77ba270de0a7a")
}

// test vectors at https://github.com/bitcoin/bips/blob/master/bip-0322.mediawiki#transaction-hashes
func TestBIP322_TxHashToSpend(t *testing.T) {
	// empty str
	toSpendTx, err := bip322.GetToSpendTx(emptyBytes, testAddrDecoded)
	require.NoError(t, err)
	require.Equal(t, "c5680aa69bb8d860bf82d4e9cd3504b55dde018de765a91bb566283c545a99a7", toSpendTx.TxHash().String())
	toSignTx := bip322.GetToSignTx(toSpendTx)
	require.Equal(t, "1e9654e951a5ba44c8604c4de6c67fd78a27e81dcadcfe1edf638ba3aaebaed6", toSignTx.TxHash().String())

	// hello world str
	toSpendTx, err = bip322.GetToSpendTx(helloWorldBytes, testAddrDecoded)
	require.NoError(t, err)
	require.Equal(t, "b79d196740ad5217771c1098fc4a4b51e0535c32236c71f1ea4d61a2d603352b", toSpendTx.TxHash().String())
	toSignTx = bip322.GetToSignTx(toSpendTx)
	require.Equal(t, "88737ae86f2077145f93cc4b153ae9a1cb8d56afa511988c149c5c8c9d93bddf", toSignTx.TxHash().String())
}

func TestBIP322_Verify(t *testing.T) {
	sigBase64 := "AkcwRAIgbAFRpM0rhdBlXr7qe5eEf3XgSeausCm2XTmZVxSYpcsCIDcbR87wF9DTrvdw1czYEEzOjso52dOSaw8VrC4GgzFRASECO5NGNFlPClJnTHNDW94h7pPL5D7xbl6FBNTrGaYpYcA="
	msgBase64 := "HRQD77+9dmnvv71N77+9O2/Wuzbvv73vv71a77+977+977+977+9Du+/ve+/vTgrNH/vv71lQX0="
	// TODO: make it work with the public key??
	address := "tb1qfwtfzdagj7efph6zfcv68ce3v48c8e9fatunur"
	addressDecoded, err := btcutil.DecodeAddress(address, net)
	require.NoError(t, err)

	emptyBytesSig, err := base64.StdEncoding.DecodeString(sigBase64)
	require.NoError(t, err)

	msg, err := base64.StdEncoding.DecodeString(msgBase64)
	require.NoError(t, err)

	witness, err := bip322.SimpleSigToWitness(emptyBytesSig)
	require.NoError(t, err)

	err = bip322.VerifyP2WPKHAndP2TR(msg, witness, addressDecoded, net)
	require.NoError(t, err)
}

func FuzzBip322ValidP2WPKHSignature(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		privkey, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		dataLen := r.Int31n(200) + 1
		dataToSign := datagen.GenRandomByteArray(r, uint64(dataLen))
		address, witness, err := datagen.SignWithP2WPKHAddress(dataToSign, privkey, net)
		require.NoError(t, err)
		witnessDecoded, err := bip322.SimpleSigToWitness(witness)
		require.NoError(t, err)

		err = bip322.VerifyP2WPKHAndP2TR(
			dataToSign,
			witnessDecoded,
			address,
			net,
		)
		require.NoError(t, err)
	})
}

func FuzzBip322ValidP2TrSpendSignature(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		privkey, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		dataLen := r.Int31n(200) + 1
		dataToSign := datagen.GenRandomByteArray(r, uint64(dataLen))
		address, witness, err := datagen.SignWithP2TrSpendAddress(dataToSign, privkey, net)
		require.NoError(t, err)
		witnessDecoded, err := bip322.SimpleSigToWitness(witness)
		require.NoError(t, err)

		err = bip322.VerifyP2WPKHAndP2TR(
			dataToSign,
			witnessDecoded,
			address,
			net,
		)
		require.NoError(t, err)
	})
}

func FuzzBip322SigHashTypeP2WPKH(f *testing.F) {
	// Add corpus entries: seed and sigHashType
	// Valid SIGHASH types
	f.Add(int64(1), uint8(0x01)) // SIGHASH_ALL - valid
	// Invalid SIGHASH types
	f.Add(int64(2), uint8(0x02)) // SIGHASH_NONE
	f.Add(int64(3), uint8(0x03)) // SIGHASH_SINGLE
	f.Add(int64(4), uint8(0x81)) // SIGHASH_ALL_ANYONECANPAY
	f.Add(int64(5), uint8(0x82)) // SIGHASH_NONE_ANYONECANPAY
	f.Add(int64(6), uint8(0x83)) // SIGHASH_SINGLE_ANYONECANPAY

	f.Fuzz(func(t *testing.T, seed int64, sigHashTypeByte uint8) {
		r := rand.New(rand.NewSource(seed))
		privkey, err := btcec.NewPrivateKey()
		require.NoError(t, err)

		dataLen := r.Int31n(200) + 1
		dataToSign := datagen.GenRandomByteArray(r, uint64(dataLen))

		sigHashType := txscript.SigHashType(sigHashTypeByte)
		address, witness, err := datagen.SignWithP2WPKHAddressWithSigHashType(
			dataToSign,
			privkey,
			net,
			sigHashType,
		)

		// btcd's WitnessSignature does not validate SIGHASH types during signing for P2WPKH
		// It allows any SIGHASH type to be signed, leaving validation to script execution
		// Therefore, signing should ALWAYS succeed regardless of SIGHASH type
		require.NoError(t, err, "P2WPKH signing should always succeed with any sighash type, got error for 0x%02x: %v", sigHashTypeByte, err)

		witnessDecoded, err := bip322.SimpleSigToWitness(witness)
		require.NoError(t, err)

		err = bip322.VerifyP2WPKHAndP2TR(
			dataToSign,
			witnessDecoded,
			address,
			net,
		)

		// BIP-322 requires SIGHASH_ALL for P2WPKH
		// btcd allowed signing with any sighash type (no validation during signing)
		// Our verification layer must enforce the BIP-322 requirement
		if sigHashType == txscript.SigHashAll {
			require.NoError(t, err, "BIP-322 should accept SIGHASH_ALL (0x01) for P2WPKH")
		} else {
			// btcd allowed signing, but our BIP-322 verification must reject
			require.Error(t, err, "BIP-322 should reject sighash type 0x%02x for P2WPKH (only SIGHASH_ALL is allowed)", sigHashTypeByte)
			require.Contains(t, err.Error(), "sighash validation failed")
		}
	})
}

func FuzzBip322SigHashTypeP2TR(f *testing.F) {
	// Add corpus entries: seed and sigHashType
	// Valid SIGHASH types
	f.Add(int64(1), uint8(0x00)) // SIGHASH_DEFAULT - valid
	f.Add(int64(2), uint8(0x01)) // SIGHASH_ALL - valid
	// Invalid SIGHASH types
	f.Add(int64(3), uint8(0x02)) // SIGHASH_NONE
	f.Add(int64(4), uint8(0x03)) // SIGHASH_SINGLE
	f.Add(int64(5), uint8(0x81)) // SIGHASH_ALL_ANYONECANPAY
	f.Add(int64(6), uint8(0x82)) // SIGHASH_NONE_ANYONECANPAY
	f.Add(int64(7), uint8(0x83)) // SIGHASH_SINGLE_ANYONECANPAY

	f.Fuzz(func(t *testing.T, seed int64, sigHashTypeByte uint8) {
		r := rand.New(rand.NewSource(seed))
		privkey, err := btcec.NewPrivateKey()
		require.NoError(t, err)

		dataLen := r.Int31n(200) + 1
		dataToSign := datagen.GenRandomByteArray(r, uint64(dataLen))

		sigHashType := txscript.SigHashType(sigHashTypeByte)
		address, witness, err := datagen.SignWithP2TrSpendAddressWithSigHashType(
			dataToSign,
			privkey,
			net,
			sigHashType,
		)

		// btcd's TaprootWitnessSignature validates SIGHASH types according to BIP 341
		// Valid Taproot sighash types are: 0x00-0x03 (DEFAULT, ALL, NONE, SINGLE)
		// and 0x81-0x83 (with ANYONECANPAY flag)
		isValidTaprootSigHash := (sigHashTypeByte <= 0x03) ||
			(sigHashTypeByte >= 0x81 && sigHashTypeByte <= 0x83)

		if isValidTaprootSigHash {
			// btcd should allow signing with any valid BIP 341 sighash type
			require.NoError(t, err, "Signing with valid Taproot sighash type 0x%02x should succeed: %v", sigHashTypeByte, err)
		} else {
			// btcd should reject invalid sighash types during signing for Taproot
			require.Error(t, err, "btcd should reject invalid Taproot sighash type 0x%02x during signing", sigHashTypeByte)
			// Can't test our verification layer if btcd rejects during signing
			return
		}

		witnessDecoded, err := bip322.SimpleSigToWitness(witness)
		require.NoError(t, err)

		err = bip322.VerifyP2WPKHAndP2TR(
			dataToSign,
			witnessDecoded,
			address,
			net,
		)

		// BIP-322 is more restrictive than BIP 341 for Taproot
		// BIP 341 allows: 0x00-0x03, 0x81-0x83 (btcd validated this during signing)
		// BIP-322 only allows: 0x00 (DEFAULT) and 0x01 (ALL)
		// Our verification layer must reject all other sighash types
		if sigHashType == txscript.SigHashDefault || sigHashType == txscript.SigHashAll {
			require.NoError(t, err, "BIP-322 should accept SIGHASH_DEFAULT (0x00) and SIGHASH_ALL (0x01) for P2TR")
		} else {
			// btcd allowed signing (valid BIP 341), but our BIP-322 verification must reject
			require.Error(t, err, "BIP-322 should reject sighash type 0x%02x for P2TR (even though it's valid in BIP 341)", sigHashTypeByte)
			require.Contains(t, err.Error(), "sighash validation failed")
		}
	})
}
