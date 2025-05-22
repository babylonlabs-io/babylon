package schnorr_adaptor_signature_test

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/stretchr/testify/require"
)

const TrueStr = "TRUE"

func executePresigVector(secKeyHex, pubKeyHex, auxRandHex, msgHex, adaptorHex, presigHex string) error {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return err
	}
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}

	msgBytes, err := hex.DecodeString(msgHex)
	if err != nil {
		return err
	}

	adaptorBytes, err := hex.DecodeString(adaptorHex)
	if err != nil {
		return err
	}
	adaptorPoint, err := btcec.ParseJacobian(adaptorBytes)
	if err != nil {
		return err
	}
	encKey, err := asig.NewEncryptionKeyFromJacobianPoint(&adaptorPoint)
	if err != nil {
		return err
	}

	if secKeyHex != "" {
		secKeyBytes, err := hex.DecodeString(secKeyHex)
		if err != nil {
			return err
		}
		secKey, _ := btcec.PrivKeyFromBytes(secKeyBytes)

		auxRandBytes, err := hex.DecodeString(auxRandHex)
		if err != nil {
			return err
		}

		sig, err := asig.EncSignWithAuxData(secKey, encKey, msgBytes, auxRandBytes)
		if err != nil {
			return err
		}

		expectedSig, err := hex.DecodeString(presigHex)
		if err != nil {
			return err
		}
		sigBytes, err := sig.Marshal()
		if err != nil {
			return err
		}
		if !bytes.Equal(expectedSig, sigBytes) {
			return err
		}
	}

	presigBytes, err := hex.DecodeString(presigHex)
	if err != nil {
		return err
	}
	presig, err := asig.NewAdaptorSignatureFromBytes(presigBytes)
	if err != nil {
		return err
	}

	return presig.EncVerify(pubKey, encKey, msgBytes)
}

func TestPresigVectors(t *testing.T) {
	f, err := os.Open(filepath.Join("vectors", "presig_vectors.csv"))
	require.NoError(t, err)
	defer f.Close()

	reader := csv.NewReader(f)
	// Skip header
	_, err = reader.Read()
	require.NoError(t, err)

	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if row[0] == "" {
			continue
		}

		index, secKeyHex, pubKeyHex, auxRandHex, msgHex, adaptorHex, presigHex, resultStr, _ := row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8]

		t.Run("Vector "+index, func(t *testing.T) {
			err := executePresigVector(secKeyHex, pubKeyHex, auxRandHex, msgHex, adaptorHex, presigHex)
			expectedResult := resultStr == TrueStr
			if expectedResult {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func executeAdaptVector(pubKeyHex, msgHex, secAdaptorHex, presigHex, bip340sigHex string) error {
	secAdaptorBytes, err := hex.DecodeString(secAdaptorHex)
	if err != nil {
		return err
	}
	var secAdaptorScalar btcec.ModNScalar
	secAdaptorScalar.SetByteSlice(secAdaptorBytes)
	decKey := &asig.DecryptionKey{ModNScalar: secAdaptorScalar}

	presigBytes, err := hex.DecodeString(presigHex)
	if err != nil {
		return err
	}
	presig, err := asig.NewAdaptorSignatureFromBytes(presigBytes)
	if err != nil {
		return err
	}

	sig, err := presig.Decrypt(decKey)
	if err != nil {
		return err
	}

	expectedSig, err := hex.DecodeString(bip340sigHex)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedSig, sig.Serialize()) {
		return fmt.Errorf("signature does not match expected")
	}

	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return err
	}
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}

	msgBytes, err := hex.DecodeString(msgHex)
	if err != nil {
		return err
	}

	if !sig.Verify(msgBytes, pubKey) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

func TestAdaptVectors(t *testing.T) {
	f, err := os.Open(filepath.Join("vectors", "adapt_vectors.csv"))
	require.NoError(t, err)
	defer f.Close()

	reader := csv.NewReader(f)
	// Skip header
	_, err = reader.Read()
	require.NoError(t, err)

	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if row[0] == "" {
			continue
		}

		index, pubKeyHex, msgHex, secAdaptorHex, presigHex, bip340sigHex, resultStr, _ := row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7]

		t.Run("Vector "+index, func(t *testing.T) {
			err := executeAdaptVector(pubKeyHex, msgHex, secAdaptorHex, presigHex, bip340sigHex)
			expectedResult := resultStr == TrueStr
			if expectedResult {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func executeSecAdaptorVector(presigHex, bip340sigHex, secAdaptorHex string) error {
	presigBytes, err := hex.DecodeString(presigHex)
	if err != nil {
		return err
	}
	presig, err := asig.NewAdaptorSignatureFromBytes(presigBytes)
	if err != nil {
		return err
	}

	bip340sigBytes, err := hex.DecodeString(bip340sigHex)
	if err != nil {
		return err
	}
	bip340sig, err := schnorr.ParseSignature(bip340sigBytes)
	if err != nil {
		return err
	}

	expectedSecAdaptor, err := hex.DecodeString(secAdaptorHex)
	if err != nil {
		return err
	}

	extractedSecAdaptor, err := presig.Extract(bip340sig)
	if err != nil {
		return err
	}

	adaptorBytes := extractedSecAdaptor.ModNScalar.Bytes()
	expectedAdaptorBytes := expectedSecAdaptor
	if !bytes.Equal(adaptorBytes[:], expectedAdaptorBytes) {
		return fmt.Errorf(
			"extracted secadaptor does not match expected: %s != %s",
			hex.EncodeToString(adaptorBytes[:]),
			hex.EncodeToString(expectedAdaptorBytes),
		)
	}

	return nil
}

func TestSecAdaptorVectors(t *testing.T) {
	f, err := os.Open(filepath.Join("vectors", "secadaptor_vectors.csv"))
	require.NoError(t, err)
	defer f.Close()

	reader := csv.NewReader(f)
	// Skip header
	_, err = reader.Read()
	require.NoError(t, err)

	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if row[0] == "" {
			continue
		}

		index, presigHex, bip340sigHex, secAdaptorHex, resultStr, _ := row[0], row[1], row[2], row[3], row[4], row[5]

		t.Run("Vector "+index, func(t *testing.T) {
			err := executeSecAdaptorVector(presigHex, bip340sigHex, secAdaptorHex)
			expectedResult := resultStr == TrueStr
			if expectedResult {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
