package types_test

import (
	crypto_rand "crypto/rand"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"

	"github.com/stretchr/testify/require"
)

func TestValidatorWithBlsKeySetValidate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name      string
		numPks    int
		setup     func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey)
		expectErr error
	}{
		{
			name:      "valid - unique addresses and keys",
			numPks:    int(datagen.RandomIntOtherThan(r, 0, 10)),
			setup:     func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {},
			expectErr: nil,
		},
		{
			name:   "duplicate validator address",
			numPks: int(datagen.RandomIntOtherThan(r, 0, 10)) + 1,
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				l := len(vs.ValSet)
				vs.ValSet[l-1].ValidatorAddress = vs.ValSet[0].ValidatorAddress
			},
			expectErr: errors.New("duplicate ValidatorAddress found"),
		},
		{
			name:   "duplicate BLS pub key",
			numPks: 2,
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				l := len(vs.ValSet)
				vs.ValSet[l-1].BlsPubKey = pks[0].PubKey()
			},
			expectErr: errors.New("duplicate BlsPubKey found"),
		},
		{
			name:   "invalid BLS pub key length",
			numPks: int(datagen.RandomIntOtherThan(r, 0, 10)),
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				vs.ValSet[0].BlsPubKey = []byte{0x01, 0x02}
			},
			expectErr: fmt.Errorf("invalid BLS public key length, got 2, expected 96"),
		},
		{
			name:   "invalid BLS pub key - not a valid point on curve",
			numPks: 1,
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				// Create a random invalid key
				invalidKey := make([]byte, bls12381.PubKeySize)
				_, err := crypto_rand.Read(invalidKey)
				require.NoError(t, err)
				vs.ValSet[0].BlsPubKey = invalidKey
			},
			expectErr: errors.New("invalid BLS public key point on the bls12-381 curve"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vs, privKeys := datagen.GenerateValidatorSetWithBLSPrivKeys(tc.numPks)
			tc.setup(vs, privKeys)
			err := vs.Validate()
			if tc.expectErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectErr.Error())
		})
	}
}

func TestBlsKeyValidateBasic(t *testing.T) {
	t.Parallel()

	validBlsKey := datagen.GenerateGenesisKey().BlsKey
	tcs := []struct {
		title string

		key    types.BlsKey
		expErr error
	}{
		{
			"valid",
			*validBlsKey,
			nil,
		},
		{
			"invalid: nil pop",
			types.BlsKey{
				Pubkey: validBlsKey.Pubkey,
				Pop:    nil,
			},
			errors.New("BLS Proof of Possession is nil"),
		},
		{
			"invalid: nil pubkey",
			types.BlsKey{
				Pubkey: nil,
				Pop:    validBlsKey.Pop,
			},
			errors.New("BLS Public key is nil"),
		},
		{
			"invalid: not a valid point on curve",
			types.BlsKey{
				Pubkey: func() *bls12381.PublicKey {
					// Create a random invalid key
					invalidKey := make([]byte, bls12381.PubKeySize)
					_, err := crypto_rand.Read(invalidKey)
					require.NoError(t, err)
					pk := new(bls12381.PublicKey)
					err = pk.Unmarshal(invalidKey)
					require.NoError(t, err)
					return pk
				}(),
				Pop: validBlsKey.Pop,
			},
			errors.New("invalid BLS public key point on the bls12-381 curve"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			actErr := tc.key.ValidateBasic()
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}
