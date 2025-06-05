package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func TestLargestBtcReOrg_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testCases := []struct {
		name    string
		lbr     types.LargestBtcReOrg
		wantErr string
	}{
		{
			name:    "nil rollback_from",
			lbr:     types.LargestBtcReOrg{RollbackFrom: nil, RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100)},
			wantErr: "rollback_from is nil",
		},
		{
			name:    "nil rollback_to",
			lbr:     types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100), RollbackTo: nil},
			wantErr: "rollback_to is nil",
		},
		{
			name: "rollback_from height lower than rollback_to",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    10,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 90),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "rollback_from height 90 is lower or equal than rollback_to height 100",
		},
		{
			name: "rollback_from height equal to rollback_to",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    0,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "rollback_from height 100 is lower or equal than rollback_to height 100",
		},
		{
			name: "block_diff does not match height difference - too small",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    40,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "block_diff 40 does not match the difference between rollback_from height 150 and rollback_to height 100 (expected 50)",
		},
		{
			name: "block_diff does not match height difference - too large",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    60,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "block_diff 60 does not match the difference between rollback_from height 150 and rollback_to height 100 (expected 50)",
		},
		{
			name: "valid rollback with correct block_diff",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    50,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "",
		},
		{
			name: "valid rollback with small height difference",
			lbr: types.LargestBtcReOrg{
				BlockDiff:    1,
				RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 101),
				RollbackTo:   datagen.GenRandomBTCHeaderInfoWithHeight(r, 100),
			},
			wantErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.lbr.Validate()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error: %q, got nil", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Errorf("expected error: %q, got %q", tc.wantErr, err.Error())
				}
			}
		})
	}
}
