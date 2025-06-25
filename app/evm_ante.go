package app

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v3/app/ante"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	evmante "github.com/cosmos/evm/ante"
	cosmosevmante "github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	cosmosevmtypes "github.com/cosmos/evm/types"
)

func NewEVMAnteHandlerOptionsFromApp(app *BabylonApp, txConfig client.TxConfig, maxGasWanted uint64) ante.EVMHandlerOptions {
	return ante.EVMHandlerOptions{
		Cdc:                    app.AppCodec(),
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: cosmosevmtypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EVMKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		FeeMarketKeeper:        app.FeemarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		TxFeeChecker:           cosmosevmante.NewDynamicFeeChecker(app.FeemarketKeeper),
	}
}

// SigVerificationGasConsumer is the Cosmos EVM implementation of SignatureVerificationGasConsumer. It consumes gas
// for signature verification based upon the public key type. The cost is fetched from the given params and is matched
// by the concrete type.
// The types of keys supported are:
//
// - ethsecp256k1 (Ethereum keys)
//
// - secp256k1 (Cosmos keys)
//
// - ed25519 (Validators)
//
// - multisig (Cosmos SDK multisigs)
func SigVerificationGasConsumer(
	meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params,
) error {
	pubkey := sig.PubKey
	switch pubkey := pubkey.(type) {
	case *ethsecp256k1.PubKey:
		// Ethereum keys
		meter.ConsumeGas(params.SigVerifyCostSecp256k1, "ante verify: eth_secp256k1")
		return nil

	case *secp256k1.PubKey:
		// Ethereum keys
		meter.ConsumeGas(params.SigVerifyCostSecp256k1, "ante verify: secp256k1")
		return nil

	case *ed25519.PubKey:
		// Validator keys
		meter.ConsumeGas(params.SigVerifyCostED25519, "ante verify: ed25519")
		return errorsmod.Wrap(errortypes.ErrInvalidPubKey, "ED25519 public keys are unsupported")

	case multisig.PubKey:
		// Multisig keys
		multisignature, ok := sig.Data.(*signing.MultiSignatureData)
		if !ok {
			return fmt.Errorf("expected %T, got, %T", &signing.MultiSignatureData{}, sig.Data)
		}
		return evmante.ConsumeMultisignatureVerificationGas(meter, multisignature, pubkey, params, sig.Sequence)

	default:
		return errorsmod.Wrapf(errortypes.ErrInvalidPubKey, "unrecognized/unsupported public key type: %T", pubkey)
	}
}
