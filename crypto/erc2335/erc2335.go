package erc2335

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

type Erc2335KeyStore struct {
	Crypto  map[string]interface{} `json:"crypto"`
	Version uint                   `json:"version"`
	UUID    string                 `json:"uuid"`
	Path    string                 `json:"path"`
	Pubkey  string                 `json:"pubkey"`
}

// wonjoon: encrypt key pair to erc2335 keystore
// available to handle all keys in []byte format
func EncryptBLS(privKey, pubKey []byte, password string) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("private key cannot be nil")
	}

	encryptor := keystorev4.New()
	cryptoFields, err := encryptor.Encrypt(privKey, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt private key")
	}

	// Create the keystore json structure
	keystoreJSON := Erc2335KeyStore{
		Crypto:  cryptoFields,
		Version: 4,
		Pubkey:  fmt.Sprintf("%x", pubKey),
	}

	return json.Marshal(keystoreJSON)
}

// decrypt private key from erc2335 keystore
func DecryptBLS(keystoreJSON []byte, password string) ([]byte, error) {
	// Parse the keystore json
	var keystore Erc2335KeyStore

	if err := json.Unmarshal(keystoreJSON, &keystore); err != nil {
		return nil, errors.Wrap(err, "failed to parse keystore json")
	}

	// Verify version
	if keystore.Version != 4 {
		return nil, fmt.Errorf("invalid keystore version: %d", keystore.Version)
	}

	encryptor := keystorev4.New()
	return encryptor.Decrypt(keystore.Crypto, password)
}
