package types_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func TestFinalityProviderSigningInfo_Validate(t *testing.T) {
	validPk := bbntypes.BIP340PubKey(make([]byte, bbntypes.BIP340PubKeyLen))
	fpPk := &validPk

	tests := []struct {
		name           string
		signingInfo    types.FinalityProviderSigningInfo
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid signing info",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             fpPk,
				StartHeight:         100,
				MissedBlocksCounter: 5,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError: false,
		},
		{
			name: "nil BTC public key",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             nil,
				StartHeight:         100,
				MissedBlocksCounter: 5,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError:    true,
			expectedErrMsg: "empty finality provider BTC public key",
		},
		{
			name: "invalid BTC public key length",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             func() *bbntypes.BIP340PubKey { k := bbntypes.BIP340PubKey([]byte{0x01, 0x02}); return &k }(),
				StartHeight:         100,
				MissedBlocksCounter: 5,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError:    true,
			expectedErrMsg: "invalid signing info. finality provider BTC public key length",
		},
		{
			name: "negative start height",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             fpPk,
				StartHeight:         -1,
				MissedBlocksCounter: 5,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError:    true,
			expectedErrMsg: "invalid start height",
		},
		{
			name: "negative missed blocks counter",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             fpPk,
				StartHeight:         100,
				MissedBlocksCounter: -1,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError:    true,
			expectedErrMsg: "invalid missed blocks counter",
		},
		{
			name: "zero values are valid",
			signingInfo: types.FinalityProviderSigningInfo{
				FpBtcPk:             fpPk,
				StartHeight:         0,
				MissedBlocksCounter: 0,
				JailedUntil:         time.Unix(0, 0).UTC(),
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.signingInfo.Validate()

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
