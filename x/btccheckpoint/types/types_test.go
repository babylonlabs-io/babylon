package types_test

import (
	"fmt"
	"testing"

	"github.com/babylonlabs-io/babylon/btctxformatter"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/stretchr/testify/require"
)

const validAddrLen = btctxformatter.AddressLength

var (
	validAddr = make([]byte, validAddrLen)
	shortAddr = make([]byte, validAddrLen-1)
	longAddr  = make([]byte, validAddrLen+1)
	emptyAddr = []byte{}
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
