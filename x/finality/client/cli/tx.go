package cli

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		NewCommitPubRandListCmd(),
		NewAddFinalitySigCmd(),
		NewUnjailFinalityProviderCmd(),
		AddEvidenceOfEquivocationCmd(),
	)

	return cmd
}

func NewCommitPubRandListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit-pubrand-list [fp_btc_pk] [start_height] [num_pub_rand] [commitment] [sig]",
		Args:  cobra.ExactArgs(5),
		Short: "Commit a list of public randomness",
		Long: strings.TrimSpace(
			`Commit a list of public randomness.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get finality provider BTC PK
			fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(args[0])
			if err != nil {
				return err
			}

			// get start height
			startHeight, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			numPubRand, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}

			commitment, err := hex.DecodeString(args[3])
			if err != nil {
				return err
			}

			// get signature
			sig, err := bbn.NewBIP340SignatureFromHex(args[4])
			if err != nil {
				return err
			}

			msg := types.MsgCommitPubRandList{
				Signer:      clientCtx.FromAddress.String(),
				FpBtcPk:     fpBTCPK,
				StartHeight: startHeight,
				NumPubRand:  numPubRand,
				Commitment:  commitment,
				Sig:         sig,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewAddFinalitySigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-finality-sig [fp_btc_pk] [block_height] [pub_rand] [proof] [block_app_hash] [finality_sig]",
		Args:  cobra.ExactArgs(6),
		Short: "Add a finality signature",
		Long: strings.TrimSpace(
			`Add a finality signature.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get finality provider BTC PK
			fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(args[0])
			if err != nil {
				return err
			}

			// get block height
			blockHeight, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			// get public randomness
			pubRand, err := bbn.NewSchnorrPubRandFromHex(args[2])
			if err != nil {
				return err
			}

			// get proof
			proofBytes, err := hex.DecodeString(args[3])
			if err != nil {
				return err
			}
			var proof cmtcrypto.Proof
			if err := clientCtx.Codec.Unmarshal(proofBytes, &proof); err != nil {
				return err
			}

			// get block app hash
			appHash, err := hex.DecodeString(args[4])
			if err != nil {
				return err
			}

			// get finality signature
			finalitySig, err := bbn.NewSchnorrEOTSSigFromHex(args[5])
			if err != nil {
				return err
			}

			msg := types.MsgAddFinalitySig{
				Signer:       clientCtx.FromAddress.String(),
				FpBtcPk:      fpBTCPK,
				BlockHeight:  blockHeight,
				PubRand:      pubRand,
				Proof:        &proof,
				BlockAppHash: appHash,
				FinalitySig:  finalitySig,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewUnjailFinalityProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unjail-finality-provider [fp_btc_pk]",
		Args:  cobra.ExactArgs(1),
		Short: "Unjail a jailed finality provider",
		Long: strings.TrimSpace(
			`Unjail a jailed finality provider.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get finality provider BTC PK
			fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(args[0])
			if err != nil {
				return err
			}

			msg := types.MsgUnjailFinalityProvider{
				Signer:  clientCtx.FromAddress.String(),
				FpBtcPk: fpBTCPK,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func AddEvidenceOfEquivocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-evidence [fp_btc_pk] [block_height] [pub_rand] [canonical_app_hash_hex] [fork_app_hash_hex] [canonical_finality_sig_hex] [fork_finality_sig_hex] [signing_context]",
		Args:  cobra.ExactArgs(8),
		Short: "Submit evidence of finality provider equivocation",
		Long: strings.TrimSpace(
			`Submit evidence that a finality provider signed conflicting blocks.
            This will slash the finality provider if the evidence is valid.
            
            Requires --signer flag to specify the signer address and --from flag for transaction signing.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			blockHeight, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			hexArgs := []struct {
				val  string
				name string
			}{
				{args[0], "fp_btc_pk"},
				{args[2], "pub_rand"},
				{args[3], "canonical_app_hash_hex"},
				{args[4], "fork_app_hash_hex"},
				{args[5], "canonical_finality_sig_hex"},
				{args[6], "fork_finality_sig_hex"},
			}
			for _, h := range hexArgs {
				if _, err := hex.DecodeString(h.val); err != nil {
					return fmt.Errorf("argument '%s' is not a valid hex string: %v", h.name, err)
				}
			}

			signer, _ := cmd.Flags().GetString("signer")
			if signer == "" {
				return fmt.Errorf("signer address is required")
			}

			msg := types.MsgEquivocationEvidence{
				Signer:                  signer,
				FpBtcPkHex:              args[0],
				BlockHeight:             blockHeight,
				PubRandHex:              args[2],
				CanonicalAppHashHex:     args[3],
				ForkAppHashHex:          args[4],
				CanonicalFinalitySigHex: args[5],
				ForkFinalitySigHex:      args[6],
				SigningContext:          args[7],
			}

			if _, err := msg.ParseToEvidence(); err != nil {
				return fmt.Errorf("failed to parse evidence: %v", err)
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String("signer", "", "signer address for the evidence (required)")
	cmd.MarkFlagRequired("signer")

	return cmd
}
