package types_test

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	time "time"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"

	"github.com/test-go/testify/require"
)

func TestValidatorWithBlsKeySetValidate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name      string
		setup     func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey)
		expectErr error
	}{
		{
			name:      "valid - unique addresses and keys",
			setup:     func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {},
			expectErr: nil,
		},
		{
			name: "duplicate validator address",
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				l := len(vs.ValSet)
				vs.ValSet[l-1].ValidatorAddress = vs.ValSet[0].ValidatorAddress
			},
			expectErr: errors.New("duplicate ValidatorAddress found"),
		},
		{
			name: "duplicate BLS pub key",
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				l := len(vs.ValSet)
				vs.ValSet[l-1].BlsPubKey = pks[0].PubKey()
			},
			expectErr: errors.New("duplicate BlsPubKey found"),
		},
		{
			name: "invalid BLS pub key length",
			setup: func(vs *types.ValidatorWithBlsKeySet, pks []bls12381.PrivateKey) {
				vs.ValSet[0].BlsPubKey = []byte{0x01, 0x02}
			},
			expectErr: fmt.Errorf("invalid BLS public key length, got 2, expected 96"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vs, privKeys := datagen.GenerateValidatorSetWithBLSPrivKeys(int(datagen.RandomIntOtherThan(r, 0, 10) + 1)) // make sure to always have at least 2 validators
			tc.setup(vs, privKeys)
			err := vs.Validate()
			if tc.expectErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr.Error())
			}
		})
	}
}
