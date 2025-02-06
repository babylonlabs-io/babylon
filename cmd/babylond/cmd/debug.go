package cmd

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	// this pkg was deprecated but still needs support
	bech32 "github.com/cosmos/cosmos-sdk/types/bech32/legacybech32" //nolint:staticcheck

	"github.com/spf13/cobra"
)

var (
	flagPubkeyType = "type"
	ed             = "ed25519"
)

// DebugCmd creates a main CLI command
func DebugCmd() *cobra.Command {
	debugCmd := debug.Cmd()

	pubKeyRawCmd := GetSubCommand(debugCmd, "pubkey-raw")
	if pubKeyRawCmd == nil {
		panic("failed to find keys pubkey-raw")
	}

	oldRun := pubKeyRawCmd.RunE
	pubKeyRawCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Run the original command
		err := oldRun(cmd, args)
		if err != nil {
			return err
		}

		return PrintBip340(cmd, args)
	}

	debugCmd.AddCommand(NewSendToRandomAddrs())
	return debugCmd
}

// PrintBip340 prints the BIP340 hex from the public key if possible
func PrintBip340(cmd *cobra.Command, args []string) error {
	pubkeyType, err := cmd.Flags().GetString(flagPubkeyType)
	if err != nil {
		return err
	}

	pk, err := getPubKeyFromRawString(args[0], pubkeyType)
	if err != nil {
		return err
	}

	bip340Key := types.BIP340PubKey(pk.Bytes())
	if err != nil {
		return err
	}

	cmd.Println("BIP340 Hex:", bip340Key.MarshalHex())
	return nil
}

// GetSubCommand returns the command if it finds, otherwise it returns nil
func GetSubCommand(cmd *cobra.Command, commandName string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if !strings.EqualFold(c.Name(), commandName) {
			continue
		}
		return c
	}
	return nil
}

// getPubKeyFromRawString returns a PubKey (PubKeyEd25519 or PubKeySecp256k1) by attempting
// to decode the pubkey string from hex, base64, and finally bech32. If all
// encodings fail, an error is returned.
// copy from https://github.com/cosmos/cosmos-sdk/blob/08fdfec9543b02ad2a72c5300ad3394916af9e02/client/debug/main.go#L142
func getPubKeyFromRawString(pkstr, keytype string) (cryptotypes.PubKey, error) {
	// Try hex decoding
	bz, err := hex.DecodeString(pkstr)
	if err == nil {
		pk, ok := bytesToPubkey(bz, keytype)
		if ok {
			return pk, nil
		}
	}

	bz, err = base64.StdEncoding.DecodeString(pkstr)
	if err == nil {
		pk, ok := bytesToPubkey(bz, keytype)
		if ok {
			return pk, nil
		}
	}

	pk, err := bech32.UnmarshalPubKey(bech32.AccPK, pkstr) //nolint:staticcheck
	if err == nil {
		return pk, nil
	}

	pk, err = bech32.UnmarshalPubKey(bech32.ValPK, pkstr) //nolint:staticcheck
	if err == nil {
		return pk, nil
	}

	pk, err = bech32.UnmarshalPubKey(bech32.ConsPK, pkstr) //nolint:staticcheck
	if err == nil {
		return pk, nil
	}

	return nil, fmt.Errorf("pubkey '%s' invalid; expected hex, base64, or bech32 of correct size", pkstr)
}

// copy from https://github.com/cosmos/cosmos-sdk/blob/08fdfec9543b02ad2a72c5300ad3394916af9e02/client/debug/main.go#L126
func bytesToPubkey(bz []byte, keytype string) (cryptotypes.PubKey, bool) {
	if keytype == ed {
		if len(bz) == ed25519.PubKeySize {
			return &ed25519.PubKey{Key: bz}, true
		}
	}

	if len(bz) == secp256k1.PubKeySize {
		return &secp256k1.PubKey{Key: bz}, true
	}
	return nil, false
}

func NewSendToRandomAddrs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bank-send-random-addrs [numberAddrs] [coins-for-each-addr]",
		Args:  cobra.ExactArgs(2),
		Short: "Create a finality provider",
		Long: strings.TrimSpace(
			`Creates a finality provider for Babylon or a Consumer chain.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			coinsForEachAddr, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return err
			}

			randomAddrs, ok := math.NewIntFromString(args[0])
			if !ok {
				return fmt.Errorf("failed to parse number from arg %s", args[0])
			}

			fromAddr := clientCtx.FromAddress

			input := banktypes.NewInput(fromAddr, coinsForEachAddr.MulInt(randomAddrs))

			numOfAddrs := randomAddrs.Uint64()
			output := make([]banktypes.Output, numOfAddrs)
			for i := uint64(0); i < numOfAddrs; i++ {
				addr := datagen.GenRandomAddress()
				output[i] = banktypes.NewOutput(addr, coinsForEachAddr)
			}

			msg := banktypes.NewMsgMultiSend(input, output)
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
