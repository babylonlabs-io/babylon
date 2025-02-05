package erc2335

import (
	"encoding/json"
	"fmt"
	"os"

	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

// Erc2335KeyStore represents an ERC-2335 compatible keystore used in keystorev4.
type Erc2335KeyStore struct {
	Crypto      map[string]interface{} `json:"crypto"`      // Map containing the encryption details for the keystore such as checksum, cipher, and kdf.
	Version     uint                   `json:"version"`     // Version of the keystore format (e.g., 4 for keystorev4).
	UUID        string                 `json:"uuid"`        // Unique identifier for the keystore.
	Path        string                 `json:"path"`        // File path where the keystore is stored.
	Pubkey      string                 `json:"pubkey"`      // Public key associated with the keystore, stored as a hexadecimal string.
	Description string                 `json:"description"` // Optional description of the keystore, currently used to store the delegator address.
}

// Encrypt encrypts the private key using the keystorev4 encryptor.
func Encrypt(privKey, pubKey []byte, password string) ([]byte, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}

	encryptor := keystorev4.New()
	cryptoFields, err := encryptor.Encrypt(privKey, password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	keystoreJSON := Erc2335KeyStore{
		Crypto:  cryptoFields,
		Version: 4,
		Pubkey:  fmt.Sprintf("%x", pubKey),
	}

	return json.Marshal(keystoreJSON)
}

// Decrypt decrypts the private key from the keystore using the given password.
func Decrypt(keystore Erc2335KeyStore, password string) ([]byte, error) {
	encryptor := keystorev4.New()
	return encryptor.Decrypt(keystore.Crypto, password)
}

// LoadKeyStore loads a keystore from a file.
func LoadKeyStore(filePath string) (Erc2335KeyStore, error) {
	var keystore Erc2335KeyStore

	keyJSONBytes, err := os.ReadFile(filePath)
	if err != nil {
		return Erc2335KeyStore{}, fmt.Errorf("failed to read keystore file: %w", err)
	}

	if err := json.Unmarshal(keyJSONBytes, &keystore); err != nil {
		return Erc2335KeyStore{}, fmt.Errorf("failed to unmarshal keystore: %w", err)
	}

	return keystore, nil
}
