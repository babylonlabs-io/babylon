package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// mockFeeTx implements the sdk.FeeTx interface for testing
type mockFeeTx struct {
	fee      sdk.Coins
	feePayer []byte
	granter  []byte
}

func (tx mockFeeTx) GetFee() sdk.Coins {
	return tx.fee
}

func (tx mockFeeTx) FeePayer() []byte {
	return tx.feePayer
}

func (tx mockFeeTx) FeeGranter() []byte {
	return tx.granter
}

// Additional methods required by sdk.FeeTx interface
func (tx mockFeeTx) GetGas() uint64     { return 0 }
func (tx mockFeeTx) GetMsgs() []sdk.Msg { return nil }
func (tx mockFeeTx) GetMsgsV2() ([]protoreflect.ProtoMessage, error) {
	return nil, nil
}

func TestRefundToCorrectRecipient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bankKeeper := types.NewMockBankKeeper(ctrl)

	iKeeper, ctx := keepertest.IncentiveKeeper(t, bankKeeper, nil, nil, nil)

	// Create test addresses
	feePayer := []byte("feePayer")
	feeGranter := []byte("feeGranter")

	mockFeeTxPayer := mockFeeTx{
		fee:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))),
		feePayer: feePayer,
		granter:  nil,
	}

	bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feePayer, gomock.Any()).Return(nil)

	err := iKeeper.RefundTx(ctx, mockFeeTxPayer)
	require.NoError(t, err)

	mockFeeTxGranter := mockFeeTx{
		fee:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))),
		feePayer: feePayer,
		granter:  feeGranter,
	}

	bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), feeGranter, gomock.Any()).Return(nil)

	err = iKeeper.RefundTx(ctx, mockFeeTxGranter)
	require.NoError(t, err)
}
