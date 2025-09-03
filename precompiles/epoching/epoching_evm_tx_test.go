package epoching_test

import (
	"context"
	"encoding/base64"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/crypto/ethsecp256k1"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
)

func (ts *PrecompileIntegrationTestSuite) TestWrappedDelegateBech32_CallContract() {
	priv, err := ethsecp256k1.GenerateKey()
	ts.Require().NoError(err)
	_, err = ts.InitAndFundEVMAccount(priv, sdkmath.NewInt(10_000_000)) // 10 bbn
	ts.Require().NoError(err)

	delegatorAcc := sdk.AccAddress(priv.PubKey().Address().Bytes())

	vals, err := ts.App.StakingKeeper.GetAllValidators(ts.Ctx)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(vals)
	valBech32 := vals[0].OperatorAddress

	amount := big.NewInt(1_000_000) // 1 bbn
	resp, err := ts.CallContract(
		priv,
		ts.addr,
		ts.abi,
		epoching.WrappedDelegateBech32Method,
		common.Address(priv.PubKey().Address().Bytes()),
		valBech32,
		amount,
	)
	ts.Require().NoError(err)
	ts.Require().Equal("", resp.VmError)

	startEpoch := ts.App.EpochingKeeper.GetEpoch(ts.Ctx).EpochNumber
	ts.AdvanceToNextEpoch()
	ts.T().Logf("epoch advanced from %d to %d at height %d", startEpoch, ts.App.EpochingKeeper.GetEpoch(ts.Ctx).EpochNumber, ts.Ctx.BlockHeight())

	delReq := &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAcc.String(),
		ValidatorAddr: valBech32,
	}
	delRes, err := ts.QueryClientStaking.Delegation(context.Background(), delReq)
	ts.Require().NoError(err)
	ts.Require().NotNil(delRes.DelegationResponse)
	ts.Require().True(delRes.DelegationResponse.Balance.Amount.GT(sdkmath.ZeroInt()))
}

func (ts *PrecompileIntegrationTestSuite) TestWrappedCreateValidator_CallContract() {
	priv, err := ethsecp256k1.GenerateKey()
	ts.Require().NoError(err)
	// fund 100 bbn for fees + self-delegation
	_, err = ts.InitAndFundEVMAccount(priv, sdkmath.NewInt(100_000_000))
	ts.Require().NoError(err)

	// consensus keys and PoP
	valPriv := cmted25519.GenPrivKey()
	blsPriv := bls12381.GenPrivKey()
	valKeys, err := appsigner.NewValidatorKeys(valPriv, blsPriv)
	ts.Require().NoError(err)

	blsKey := epoching.BlsKey{
		PubKey:     valKeys.BlsPubkey.Bytes(),
		Ed25519Sig: valKeys.PoP.Ed25519Sig,
		BlsSig:     valKeys.PoP.BlsSig.Bytes(),
	}
	desc := epoching.Description{Moniker: "evm-val"}
	comm := epoching.Commission{
		Rate:          big.NewInt(10000000000000000),
		MaxRate:       big.NewInt(100000000000000000),
		MaxChangeRate: big.NewInt(100000000000000000),
	}
	msd := big.NewInt(1)
	valHex := common.Address(priv.PubKey().Address().Bytes())
	consPkB64 := base64.StdEncoding.EncodeToString(valKeys.ValPubkey.Bytes())
	amount := big.NewInt(1_000_000) // 1 bbn

	resp, err := ts.CallContract(
		priv, ts.addr, ts.abi, epoching.WrappedCreateValidatorMethod,
		blsKey, desc, comm, msd, valHex, consPkB64, amount,
	)
	ts.Require().NoError(err)
	ts.Require().Equal("", resp.VmError)

	ts.AdvanceToNextEpoch()

	valAddr := sdk.ValAddress(valHex.Bytes())
	v, err := ts.App.StakingKeeper.GetValidator(ts.Ctx, valAddr)
	ts.Require().NoError(err)
	ts.Require().Equal(valAddr.String(), v.OperatorAddress)
	ts.Require().True(v.Tokens.IsPositive())
}
