package replay

import (
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

func (d *BabylonAppDriver) TxUnjailValidator(operator *SenderInfo, valAddr string) {
	msgUnjail := slashingtypes.NewMsgUnjail(valAddr)
	d.SendTxWithMessagesSuccess(d.t, operator, DefaultGasLimit, defaultFeeCoin, msgUnjail)
	operator.IncSeq()
}
