package tmanager

import (
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/stretchr/testify/require"
)

// NewFpWithWallet generates a fresh BTC keypair, registers a finality
// provider on Babylon under the provided wallet, and returns a
// FinalityProvider handle. Backported from main for the stake-expansion
// regression test.
//
// Populates BtcPrivKey, BtcPrivateKey (alias), and PublicKey on the
// returned FinalityProvider; the type itself is defined in wallet.go.
func (n *Node) NewFpWithWallet(wallet *WalletSender) *FinalityProvider {
	fpSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(n.T(), err)

	fp, err := datagen.GenCustomFinalityProvider(n.Tm.R, fpSK, wallet.Address)
	require.NoError(n.T(), err)

	n.CreateFinalityProvider(wallet.KeyName, fp)
	n.WaitForNextBlock()

	fpResp := n.QueryFinalityProvider(fp.BtcPk.MarshalHex())
	require.NotNil(n.T(), fpResp)

	return &FinalityProvider{
		WalletSender:  wallet,
		BtcPrivKey:    fpSK,
		BtcPrivateKey: fpSK,
		PublicKey:     fp.BtcPk,
		Commission:    *fp.Commission,
	}
}
