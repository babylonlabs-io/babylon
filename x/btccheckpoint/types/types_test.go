package types_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v2/btctxformatter"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v2/types"
	"github.com/babylonlabs-io/babylon/v2/x/btccheckpoint/types"
	"github.com/stretchr/testify/require"
)

const validAddrLen = btctxformatter.AddressLength

var (
	validAddr = make([]byte, validAddrLen)
	shortAddr = make([]byte, validAddrLen-1)
	longAddr  = make([]byte, validAddrLen+1)
	emptyAddr = []byte{}
	r         = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func TestCheckpointAddresses_Validate(t *testing.T) {
	tests := []struct {
		name      string
		addresses types.CheckpointAddresses
		wantErr   error
	}{
		{
			name: "valid addresses",
			addresses: types.CheckpointAddresses{
				Submitter: validAddr,
				Reporter:  validAddr,
			},
			wantErr: nil,
		},
		{
			name: "submitter too short",
			addresses: types.CheckpointAddresses{
				Submitter: shortAddr,
				Reporter:  validAddr,
			},
			wantErr: fmt.Errorf("invalid submitter address length: expected %d, got %d", validAddrLen, len(shortAddr)),
		},
		{
			name: "submitter too long",
			addresses: types.CheckpointAddresses{
				Submitter: longAddr,
				Reporter:  validAddr,
			},
			wantErr: fmt.Errorf("invalid submitter address length: expected %d, got %d", validAddrLen, len(longAddr)),
		},
		{
			name: "reporter too short",
			addresses: types.CheckpointAddresses{
				Submitter: validAddr,
				Reporter:  shortAddr,
			},
			wantErr: fmt.Errorf("invalid reporter address length: expected %d, got %d", validAddrLen, len(shortAddr)),
		},
		{
			name: "reporter too long",
			addresses: types.CheckpointAddresses{
				Submitter: validAddr,
				Reporter:  longAddr,
			},
			wantErr: fmt.Errorf("invalid reporter address length: expected %d, got %d", validAddrLen, len(longAddr)),
		},
		{
			name: "both invalid",
			addresses: types.CheckpointAddresses{
				Submitter: shortAddr,
				Reporter:  longAddr,
			},
			wantErr: fmt.Errorf("invalid submitter address length: expected %d, got %d", validAddrLen, len(shortAddr)),
		},
		{
			name: "both empty",
			addresses: types.CheckpointAddresses{
				Submitter: emptyAddr,
				Reporter:  emptyAddr,
			},
			wantErr: fmt.Errorf("invalid submitter address length: expected %d, got %d", validAddrLen, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.addresses.Validate()
			if tt.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTransactionKey_Validate(t *testing.T) {
	t.Parallel()
	header := datagen.GenRandomBTCHeaderInfo(r)
	tests := []struct {
		name    string
		key     types.TransactionKey
		wantErr error
	}{
		{
			name: "valid key",
			key: types.TransactionKey{
				Index: 0,
				Hash:  datagen.GenRandomBTCHeaderPrevBlock(r),
			},
			wantErr: nil,
		},
		{
			name: "valid key random",
			key: types.TransactionKey{
				Index: 0,
				Hash:  header.Hash,
			},
			wantErr: nil,
		},
		{
			name: "invalid hash",
			key: types.TransactionKey{
				Index: 0,
				Hash:  nil,
			},
			wantErr: fmt.Errorf("transaction hash cannot be nil"),
		},
		{
			name: "invalid hash length",
			key: types.TransactionKey{
				Index: 0,
				Hash:  &bbntypes.BTCHeaderHashBytes{},
			},
			wantErr: fmt.Errorf("invalid transaction hash length: expected 32, got 0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.Validate()
			if tt.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
