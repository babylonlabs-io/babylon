package chain

import (
	"encoding/json"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Copy from https://github.com/cosmos/cosmos-sdk/blob/4251905d56e0e7a3350145beedceafe786953295/x/gov/client/cli/util.go#L83
// Not exported structure and file
// Proposal defines the new Msg-based Proposal.
type Proposal struct {
	// Msgs defines an array of sdk.Msgs proto-JSON-encoded as Anys.
	Messages  []json.RawMessage `json:"messages,omitempty"`
	Metadata  string            `json:"metadata"`
	Deposit   string            `json:"deposit"`
	Title     string            `json:"title"`
	Summary   string            `json:"summary"`
	Expedited bool              `json:"expedited"`
}

// ParseSubmitProposal reads and parses the proposal.
func ParseSubmitProposal(cdc codec.Codec, path string) (Proposal, []sdk.Msg, sdk.Coins, error) {
	var proposal Proposal

	contents, err := os.ReadFile(path)
	if err != nil {
		return proposal, nil, nil, err
	}

	err = json.Unmarshal(contents, &proposal)
	if err != nil {
		return proposal, nil, nil, err
	}

	msgs := make([]sdk.Msg, len(proposal.Messages))
	for i, anyJSON := range proposal.Messages {
		var msg sdk.Msg
		err := cdc.UnmarshalInterfaceJSON(anyJSON, &msg)
		if err != nil {
			return proposal, nil, nil, err
		}

		msgs[i] = msg
	}

	deposit, err := sdk.ParseCoinsNormalized(proposal.Deposit)
	if err != nil {
		return proposal, nil, nil, err
	}

	return proposal, msgs, deposit, nil
}

// WriteProposalToFile marshal the prop as json to the file.
func WriteProposalToFile(_ codec.Codec, path string, prop Proposal) error {
	bz, err := json.MarshalIndent(&prop, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bz, 0644)
}
