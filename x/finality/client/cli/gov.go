package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/spf13/cobra"

	bbncli "github.com/babylonlabs-io/babylon/client"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

type FinalityProviderPks struct {
	FinalityProviders []string `json:"finality-providers"`
}

func NewCmdSubmitResumeFinalityProposal() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume-finality [fps-to-jail.json] [halting-height]",
		Args:  cobra.ExactArgs(2),
		Short: "Submit a resume finality proposal",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, proposalTitle, summary, deposit, isExpedited, authority, err := bbncli.GetProposalInfo(cmd)
			if err != nil {
				return err
			}

			fps, err := loadFpsFromFile(args[0])
			if err != nil {
				return fmt.Errorf("cannot load finality providers from %s: %w", args[0], err)
			}

			haltingHeight, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid halting-height %s: %w", args[1], err)
			}

			content := types.NewResumeFinalityProposal(proposalTitle, summary, fps, uint32(haltingHeight))

			contentMsg, err := v1.NewLegacyContent(content, authority.String())
			if err != nil {
				return err
			}

			msg := v1.NewMsgExecLegacyContent(contentMsg.Content, authority.String())

			proposalMsg, err := v1.NewMsgSubmitProposal([]sdk.Msg{msg}, deposit, clientCtx.GetFromAddress().String(), "", proposalTitle, summary, isExpedited)
			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), proposalMsg)
		},
	}

	bbncli.AddCommonProposalFlags(cmd)

	return cmd
}

func loadFpsFromFile(filePath string) ([]bbntypes.BIP340PubKey, error) {
	bz, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}
	fps := new(FinalityProviderPks)
	err = tmjson.Unmarshal(bz, fps)
	if err != nil {
		return nil, err
	}

	if len(fps.FinalityProviders) == 0 {
		return nil, fmt.Errorf("empty finality providers")
	}

	fpPks := make([]bbntypes.BIP340PubKey, len(fps.FinalityProviders))
	for i, pkStr := range fps.FinalityProviders {
		pk, err := bbntypes.NewBIP340PubKeyFromHex(pkStr)
		if err != nil {
			return nil, fmt.Errorf("invalid finality provider public key %s: %w", pkStr, err)
		}
		fpPks[i] = *pk
	}

	return fpPks, nil
}
