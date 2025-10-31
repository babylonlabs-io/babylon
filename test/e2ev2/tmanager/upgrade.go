package tmanager

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec/types"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

var (
	ErrInvalidUpgradeMsg   = fmt.Errorf("invalid upgrade message type")
	ErrNoProposalSubmitted = fmt.Errorf("no proposal submitted")
	ErrProposalNotPassed   = fmt.Errorf("proposal is not passed")
)

func (tm *TestManagerUpgrade) runForkUpgrade() error {
	tm.T.Logf("waiting to reach fork height on chain")
	tm.ChainsWaitUntilHeight(uint32(tm.ForkHeight))
	tm.T.Logf("fork height reached on chain")
	return nil
}

func (tm *TestManagerUpgrade) runProposalUpgrade(govMsg *govtypes.MsgSubmitProposal) error {
	// submit, deposit, and vote for upgrade proposal
	for _, chain := range tm.Chains {
		validator := chain.Validators[0]
		currentHeight, err := validator.LatestBlockNumber()
		if err != nil {
			return err
		}

		msgs, err := govMsg.GetMsgs()
		if err != nil {
			return err
		}
		upgradeMsg, ok := msgs[0].(*upgradetypes.MsgSoftwareUpgrade)
		if !ok {
			return ErrInvalidUpgradeMsg
		}

		var updatedGovMsg *govtypes.MsgSubmitProposal
		if upgradeMsg.Plan.Height <= int64(currentHeight) {
			// update govMsg giving buffer 10 block to the current height
			upgradeMsg.Plan.Height = int64(currentHeight + 10)
			chain.Config.UpgradePropHeight = upgradeMsg.Plan.Height
			updatedGovMsg, err = updateGovUpgradeMsg(govMsg, upgradeMsg.Plan)
			if err != nil {
				return err
			}
			govMsg = updatedGovMsg
		}
		chain.Config.UpgradePropHeight = upgradeMsg.Plan.Height

		// submit upgrade gov proposal and vote yes
		// force increase sequence of validator
		validator.Wallet.WalletSender.IncSeq()
		validator.SubmitProposal(validator.Wallet.KeyName, govMsg)
		validator.WaitForNextBlock()
		propsResp := validator.QueryProposals()
		if len(propsResp.Proposals) == 0 {
			return ErrNoProposalSubmitted
		}
		proposalID := propsResp.Proposals[0].Id
		tm.T.Logf("proposal %d submitted, current status: %d", proposalID, propsResp.Proposals[0].Status)
		validator.Vote(validator.Wallet.KeyName, proposalID, govtypes.VoteOption_VOTE_OPTION_YES)
		validator.WaitForNextBlock()
		tallyResult := validator.QueryTallyResult(proposalID)
		tm.T.Logf("tally result from validator: %v", tallyResult)
	}

	// wait till all chains halt at upgrade height
	for _, chain := range tm.Chains {
		tm.T.Logf("waiting to reach upgrade height on chain %s", chain.ChainID())
		chain.WaitUntilBlkHeight(uint32(chain.Config.UpgradePropHeight))
		tm.T.Logf("upgrade height %d reached on chain %s", chain.Config.UpgradePropHeight, chain.ChainID())
	}

	// check proposal status
	validator := tm.ChainValidator()
	propsResp := validator.QueryProposals()
	if propsResp.Proposals[0].Status != 3 {
		return ErrProposalNotPassed
	}

	// remove all containers so we can upgrade them to the new version
	for _, chain := range tm.Chains {
		for _, node := range chain.AllNodes() {
			if err := node.RemoveResource(); err != nil {
				return err
			}
		}
	}

	// upgrade all containers
	for _, chain := range tm.Chains {
		if err := tm.upgradeContainers(chain, chain.Config.UpgradePropHeight); err != nil {
			return err
		}
	}

	return nil
}

func (tm *TestManagerUpgrade) upgradeContainers(chain *Chain, propHeight int64) error {
	tm.T.Logf("starting upgrade for chain-if: %s...", chain.ChainID())
	// update all nodes current repository and current tag
	for _, node := range chain.AllNodes() {
		node.Container.Repository = BabylonContainerName
		node.Container.Tag = "latest"
	}

	// run chain
	chain.Start()

	tm.T.Logf("waiting to upgrade containers on chain %s", chain.ChainID())
	chain.WaitUntilBlkHeight(uint32(propHeight + 1))
	tm.T.Logf("upgrade successful on chain %s", chain.ChainID())

	return nil
}

func updateGovUpgradeMsg(govMsg *govtypes.MsgSubmitProposal, plan upgradetypes.Plan) (*govtypes.MsgSubmitProposal, error) {
	msgs, err := govMsg.GetMsgs()
	if err != nil {
		return nil, err
	}
	upgradeMsg, ok := msgs[0].(*upgradetypes.MsgSoftwareUpgrade)
	if !ok {
		return nil, ErrInvalidUpgradeMsg
	}

	upgradeMsg.Plan = plan
	anyMsg, err := types.NewAnyWithValue(upgradeMsg)
	if err != nil {
		return nil, err
	}
	govMsg.Messages = []*types.Any{anyMsg}

	return govMsg, nil
}
