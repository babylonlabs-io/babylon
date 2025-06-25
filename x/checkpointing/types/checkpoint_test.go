package types_test

import (
	"math/rand"
	"testing"
	time "time"

	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"

	"github.com/test-go/testify/require"
)

func TestRawCheckpointWithMeta_Validate(t *testing.T) {
	var (
		r              = rand.New(rand.NewSource(time.Now().Unix()))
		validPk        = bls12381.GenPrivKey().PubKey()
		invalidPk      = bls12381.PublicKey(make([]byte, bls12381.PubKeySize-1))
		validCkpt      = datagen.GenRandomRawCheckpointWithMeta(r)
		validTime      = time.Now().UTC()
		validLifecycle = []*types.CheckpointStateUpdate{
			{
				State:       types.Submitted,
				BlockHeight: 100,
				BlockTime:   &validTime,
			},
		}
	)
	validCkpt.Lifecycle = append(validCkpt.Lifecycle, validLifecycle...)
	validCkpt.BlsAggrPk = &validPk

	tests := []struct {
		name      string
		meta      types.RawCheckpointWithMeta
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid checkpoint with meta",
			meta:      *validCkpt,
			expectErr: false,
		},
		{
			name: "nil checkpoint",
			meta: types.RawCheckpointWithMeta{
				Ckpt:      nil,
				Status:    validCkpt.Status,
				BlsAggrPk: &validPk,
			},
			expectErr: true,
			errMsg:    "checkpoint is nil",
		},
		{
			name: "invalid checkpoint status",
			meta: types.RawCheckpointWithMeta{
				Ckpt:      validCkpt.Ckpt,
				Status:    types.CheckpointStatus(99),
				BlsAggrPk: &validPk,
			},
			expectErr: true,
			errMsg:    "raw checkpoint's status is invalid",
		},
		{
			name: "nil BLS aggregated pub key",
			meta: types.RawCheckpointWithMeta{
				Ckpt:      validCkpt.Ckpt,
				Status:    types.Sealed,
				BlsAggrPk: nil,
			},
			expectErr: true,
			errMsg:    "BLS aggregated pub key is nil",
		},
		{
			name: "invalid size of BLS aggregated pub key",
			meta: types.RawCheckpointWithMeta{
				Ckpt:      validCkpt.Ckpt,
				Status:    types.Sealed,
				BlsAggrPk: &invalidPk,
			},
			expectErr: true,
			errMsg:    "invalid size of BlsAggrPk",
		},
		{
			name: "invalid lifecycle entry",
			meta: types.RawCheckpointWithMeta{
				Ckpt:      validCkpt.Ckpt,
				Status:    types.Sealed,
				BlsAggrPk: &validPk,
				Lifecycle: []*types.CheckpointStateUpdate{
					{
						State:       types.Sealed,
						BlockHeight: 0, // invalid block height
						BlockTime:   &validTime,
					},
				},
			},
			expectErr: true,
			errMsg:    "block height is zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.meta.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckpointStateUpdate_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		update    types.CheckpointStateUpdate
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid state update",
			update: types.CheckpointStateUpdate{
				State:       types.Sealed,
				BlockHeight: 100,
				BlockTime:   &now,
			},
			expectErr: false,
		},
		{
			name: "invalid state",
			update: types.CheckpointStateUpdate{
				State:       99,
				BlockHeight: 100,
				BlockTime:   &now,
			},
			expectErr: true,
			errMsg:    "raw checkpoint's status is invali",
		},
		{
			name: "zero block height",
			update: types.CheckpointStateUpdate{
				State:       types.Sealed,
				BlockHeight: 0,
				BlockTime:   &now,
			},
			expectErr: true,
			errMsg:    "block height is zero",
		},
		{
			name: "nil block time",
			update: types.CheckpointStateUpdate{
				State:       types.Sealed,
				BlockHeight: 100,
				BlockTime:   nil,
			},
			expectErr: true,
			errMsg:    "block time is nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.update.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
