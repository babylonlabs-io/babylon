package epoching_test

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"

	epochingpc "github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *PrecompileIntegrationTestSuite) TestWrappedDelegateBech32_CallContract() {
	priv, err := ethsecp256k1.GenerateKey()
	s.Require().NoError(err)
	_, err = s.InitAndFundEVMAccount(priv, sdkmath.NewInt(10_000_000)) // 10 bbn
	s.Require().NoError(err)

	delegatorAcc := sdk.AccAddress(priv.PubKey().Address().Bytes())

	vals, err := s.App.StakingKeeper.GetAllValidators(s.Ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(vals)
	valBech32 := vals[0].OperatorAddress

	amount := big.NewInt(1_000_000) // 1 bbn
	resp, err := s.CallContract(
		priv,
		s.addr,
		s.abi,
		epochingpc.WrappedDelegateBech32Method,
		common.Address(priv.PubKey().Address().Bytes()),
		valBech32,
		amount,
	)
	s.Require().NoError(err)
	s.Require().Equal("", resp.VmError)

	startEpoch := s.App.EpochingKeeper.GetEpoch(s.Ctx).EpochNumber
	for !s.App.EpochingKeeper.GetEpoch(s.Ctx).IsLastBlock(s.Ctx) {
		s.Commit(nil)
	}
	s.Commit(nil)
	s.T().Logf("epoch advanced from %d to %d at height %d", startEpoch, s.App.EpochingKeeper.GetEpoch(s.Ctx).EpochNumber, s.Ctx.BlockHeight())

	delReq := &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAcc.String(),
		ValidatorAddr: valBech32,
	}
	delRes, err := s.QueryClientStaking.Delegation(context.Background(), delReq)
	s.Require().NoError(err)
	s.Require().NotNil(delRes.DelegationResponse)
	s.Require().True(delRes.DelegationResponse.Balance.Amount.GT(sdkmath.ZeroInt()))
}
