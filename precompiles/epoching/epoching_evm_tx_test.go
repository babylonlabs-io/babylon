package epoching_test

import (
	"context"
	"encoding/base64"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	sdkmath "cosmossdk.io/math"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
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
		epoching.WrappedDelegateBech32Method,
		common.Address(priv.PubKey().Address().Bytes()),
		valBech32,
		amount,
	)
	s.Require().NoError(err)
	s.Require().Equal("", resp.VmError)

	startEpoch := s.App.EpochingKeeper.GetEpoch(s.Ctx).EpochNumber
	s.AdvanceToNextEpoch()
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

func (s *PrecompileIntegrationTestSuite) TestWrappedCreateValidator_CallContract() {
	priv, err := ethsecp256k1.GenerateKey()
	s.Require().NoError(err)
	// fund 100 bbn for fees + self-delegation
	_, err = s.InitAndFundEVMAccount(priv, sdkmath.NewInt(100_000_000))
	s.Require().NoError(err)

	// consensus keys and PoP
	valPriv := cmted25519.GenPrivKey()
	blsPriv := bls12381.GenPrivKey()
	valKeys, err := appsigner.NewValidatorKeys(valPriv, blsPriv)
	s.Require().NoError(err)

	blsKey := epoching.BlsKey{
		PubKey:     valKeys.BlsPubkey.Bytes(),
		Ed25519Sig: valKeys.PoP.Ed25519Sig,
		BlsSig:     valKeys.PoP.BlsSig.Bytes(),
	}
	desc := epoching.Description{Moniker: "evm-val"}
	zero := big.NewInt(0)
	comm := epoching.Commission{Rate: zero, MaxRate: zero, MaxChangeRate: zero}
	msd := big.NewInt(1)
	valHex := common.Address(priv.PubKey().Address().Bytes())
	consPkB64 := base64.StdEncoding.EncodeToString(valKeys.ValPubkey.Bytes())
	amount := big.NewInt(1_000_000) // 1 bbn

	resp, err := s.CallContract(
		priv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
		blsKey, desc, comm, msd, valHex, consPkB64, amount,
	)
	s.Require().NoError(err)
	s.Require().Equal("", resp.VmError)

	s.AdvanceToNextEpoch()

	valAddr := sdk.ValAddress(valHex.Bytes())
	v, err := s.App.StakingKeeper.GetValidator(s.Ctx, valAddr)
	s.Require().NoError(err)
	s.Require().Equal(valAddr.String(), v.OperatorAddress)
	s.Require().True(v.Tokens.IsPositive())
}
