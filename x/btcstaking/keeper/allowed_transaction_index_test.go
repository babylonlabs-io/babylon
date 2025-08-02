package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func TestIsMultiStakingAllowed(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set all parameters with custom max finality providers to allow multi-staking
	// disable regular allow list (set to 0)
	h.GenAndApplyCustomParams(r, 100, 200, 0, 2)

	// start at block height 2 (within multi-staking allow-list period since placeholder sets it to 5)
	h.Ctx = h.Ctx.WithBlockHeight(2)

	// generate test data
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// create finality providers - one Babylon FP and one Consumer FP
	_, babylonFPPK, _ := h.CreateFinalityProvider(r)

	// register a consumer and create a consumer FP
	consumer := h.RegisterAndVerifyConsumer(t, r)
	_, consumerFPPK, _, err := h.CreateConsumerFinalityProvider(r, consumer.ConsumerId)
	require.NoError(t, err)

	// create initial delegation (single Babylon FP)
	stakingTxHashStr, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{babylonFPPK},
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
	)
	require.NoError(t, err)
	txHashInAllowList, err := chainhash.NewHashFromStr(stakingTxHashStr)
	require.NoError(t, err)

	// create multi-staking delegation for testing (Babylon FP + Consumer FP)
	// need to go beyond allow-list period to create regular multi-staking delegation
	h.Ctx = h.Ctx.WithBlockHeight(6) // beyond multiStakingAllowListExpirationHeight = 5
	multiStakingTxHashStr, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{babylonFPPK, consumerFPPK},
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
	)
	require.NoError(t, err)
	multiStakingTxHash, err := chainhash.NewHashFromStr(multiStakingTxHashStr)
	require.NoError(t, err)

	// go back to allow-list period for testing
	h.Ctx = h.Ctx.WithBlockHeight(2)

	// add regular delegation to multi-staking allow list
	h.BTCStakingKeeper.IndexAllowedMultiStakingTransaction(h.Ctx, txHashInAllowList)

	testCases := []struct {
		name        string
		setupFunc   func() *types.ParsedCreateDelegationMessage
		isAllowed   bool
		expectedErr string
	}{
		{
			name: "nil parsed message - not allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				return nil
			},
			isAllowed:   false,
			expectedErr: "it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period",
		},
		{
			name: "no stake expansion - not allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue),
					StkExp:       nil,
				}
			},
			isAllowed:   false,
			expectedErr: "it is not allowed to create new delegations with multi-staking during the multi-staking allow-list period",
		},
		{
			name: "previous tx in allow list - allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue),
					StkExp: &types.ParsedCreateDelStkExp{
						PreviousActiveStkTxHash: txHashInAllowList,
					},
				}
			},
			isAllowed:   true,
			expectedErr: "",
		},
		{
			name: "previous tx is multi-staking (not in allow list) - allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue),
					StkExp: &types.ParsedCreateDelStkExp{
						PreviousActiveStkTxHash: multiStakingTxHash,
					},
				}
			},
			isAllowed:   true,
			expectedErr: "",
		},
		{
			name: "previous tx not in allow list and single FP (regular delegation) - not allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				// create another regular delegation not in allow list
				nonAllowedTxHashStr, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					[]*btcec.PublicKey{babylonFPPK},
					stakingValue,
					1000,
					0,
					0,
					false,
					false,
					10,
					30,
				)
				require.NoError(t, err)
				nonAllowedTxHash, err := chainhash.NewHashFromStr(nonAllowedTxHashStr)
				require.NoError(t, err)

				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue),
					StkExp: &types.ParsedCreateDelStkExp{
						PreviousActiveStkTxHash: nonAllowedTxHash,
					},
				}
			},
			isAllowed:   false,
			expectedErr: "",
		},
		{
			name: "staking amount changed - not allowed",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue + 1000), // different amount
					StkExp: &types.ParsedCreateDelStkExp{
						PreviousActiveStkTxHash: txHashInAllowList,
					},
				}
			},
			isAllowed:   false,
			expectedErr: "it is not allowed to modify the staking amount during the multi-staking allow-list period",
		},
		{
			name: "non-existent previous tx hash - error",
			setupFunc: func() *types.ParsedCreateDelegationMessage {
				nonExistentHash, err := chainhash.NewHash(datagen.GenRandomByteArray(r, 32))
				require.NoError(t, err)
				return &types.ParsedCreateDelegationMessage{
					StakingValue: btcutil.Amount(stakingValue),
					StkExp: &types.ParsedCreateDelStkExp{
						PreviousActiveStkTxHash: nonExistentHash,
					},
				}
			},
			isAllowed:   false,
			expectedErr: "failed to find BTC delegation for tx hash",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedMsg := tc.setupFunc()

			result, err := h.BTCStakingKeeper.IsMultiStakingAllowed(h.Ctx, parsedMsg)

			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.isAllowed, result)
		})
	}
}
