package types_test

import (
	"math/rand"
	"testing"
	time "time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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
			name:    "rollback_from height lower than rollback_to",
			lbr:     types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 90), RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100)},
			wantErr: "rollback_from height 90 is lower or equal than rollback_to height 100",
		},
		{
			name:    "rollback_from height equal to rollback_to",
			lbr:     types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100), RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100)},
			wantErr: "rollback_from height 100 is lower or equal than rollback_to height 100",
		},
		{
			name:    "valid rollback",
			lbr:     types.LargestBtcReOrg{RollbackFrom: datagen.GenRandomBTCHeaderInfoWithHeight(r, 150), RollbackTo: datagen.GenRandomBTCHeaderInfoWithHeight(r, 100)},
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
