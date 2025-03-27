package types_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/test-go/testify/require"
)

var (
	MSG1 = `{"msg": "{\"signer\":\"bbn1llns56v0844chclvtsnttnjmpxny40sn64unt3\",\"fp_btc_pk\":\"2877658a1e7608fe84543a75ca879334bb8f2e7aecae71adb7767192bda50739\",\"block_height\":595991,\"pub_rand\":\"rZEYMlskTzfZp2gL0hBmSt6cFMt8cE4IFW/XWDT7KrI=\",\"proof\":{\"total\":70000,\"index\":27147,\"leaf_hash\":\"QlJIdApSjYva+cxebPlJLwYbeDdZIG4/cKSbFK486rY=\",\"aunts\":[\"Ny5x4THidh6I6eDcH+qrM2TekVYocr1jmfvK4uNbogU=\",\"b3D3s4SZuTKznkLvjvqt0+Qvxy6Wrc0cS75/NpCXwfE=\",\"qFLP5fG5zuhgYAr1QGwQTaAOa3fNohEt99x0WTxERSg=\",\"3TaxNa6G5F6K/T3w0EcXinRfHMtInthnfzWhcBZ/i0k=\",\"JUYdT96QwZNxb2cyisY62nQ+oglmuak8fJXhFMgXj6g=\",\"eMQMyUubJX8MWYUtsxSq5qOHg5ximWd6h7hrt1PSOxM=\",\"Al75nCG9D6KHCIlQBinulrmoqf4zziCX1s1KrvMgSzM=\",\"v4Ep+U2Q5I/vUlx77brPgJESIZHCMKERV0hQqRwUYKk=\",\"9xtU7fYolEW0+6Tl8d+1G8WM+XkchhZ9wPGaUQ39bT8=\",\"/WkTLgS47o0vsNn6aeK4XtWui0+S/gdGB0qrzhQfu28=\",\"6Fm78utdm1M1W0DrCKPp1E6edDhvEy+ssCKG1xLRZvE=\",\"BbZ68FXOCHk7i2Z75VnMtugQPDK9wH6ioOVSZUyatpM=\",\"9U/alqJcsDpqjZCDnRnlbjYK4SsqwPdU8xoC2nQxZlw=\",\"EfajX5SWuW2YREjBMlzBfLhdgp+O8ZWRCcbMvsY0M+I=\",\"cdp93LsDsyqku102jI/DaHcxDnIkLDSwR6PNNs/Djtg=\",\"1m1ztn/R0T8tqtH0iVFMPF2JB+PmvkDdEdNWjiu3Qug=\",\"FI7Baxp7Q3kY6W+uQyPptLlLDeK7O6eBdWGg7fYw9W8=\"]},\"block_app_hash\":\"pUmu/qlxb1QfNlv5TJSkxXGtiAXMsuTB3Asb8htZhvQ=\",\"finality_sig\":\"GiZplD0lpaHu1srjJxfmAJx52sj9uXCU2uHYwytHHVA=\"}"}
	`

	MSG2 = `{"msg": "{\"signer\":\"bbn1llns56v0844chclvtsnttnjmpxny40sn64unt3\",\"fp_btc_pk\":\"2877658a1e7608fe84543a75ca879334bb8f2e7aecae71adb7767192bda50739\",\"block_height\":595992,\"pub_rand\":\"RrBkeGyIcHWzEhypnsWfKJHaWiVjXcRz92r1h206+zg=\",\"proof\":{\"total\":70000,\"index\":27148,\"leaf_hash\":\"CwoZFuS7U7rMcB34FbYjE86gVsH1kRmK5cVZ1HLFalo=\",\"aunts\":[\"71m1CuA2KFTchCMlf/tFJCmmOt/k/yTuA0Dpf+afqgQ=\",\"NCKhl9MsEawZ780KGn4sfqTF3Bo43wXXcW+LgSDybY4=\",\"59sqIWfTeAHZznLN6zN2lPApzamkQptwsTRO17BTVoM=\",\"3TaxNa6G5F6K/T3w0EcXinRfHMtInthnfzWhcBZ/i0k=\",\"JUYdT96QwZNxb2cyisY62nQ+oglmuak8fJXhFMgXj6g=\",\"eMQMyUubJX8MWYUtsxSq5qOHg5ximWd6h7hrt1PSOxM=\",\"Al75nCG9D6KHCIlQBinulrmoqf4zziCX1s1KrvMgSzM=\",\"v4Ep+U2Q5I/vUlx77brPgJESIZHCMKERV0hQqRwUYKk=\",\"9xtU7fYolEW0+6Tl8d+1G8WM+XkchhZ9wPGaUQ39bT8=\",\"/WkTLgS47o0vsNn6aeK4XtWui0+S/gdGB0qrzhQfu28=\",\"6Fm78utdm1M1W0DrCKPp1E6edDhvEy+ssCKG1xLRZvE=\",\"BbZ68FXOCHk7i2Z75VnMtugQPDK9wH6ioOVSZUyatpM=\",\"9U/alqJcsDpqjZCDnRnlbjYK4SsqwPdU8xoC2nQxZlw=\",\"EfajX5SWuW2YREjBMlzBfLhdgp+O8ZWRCcbMvsY0M+I=\",\"cdp93LsDsyqku102jI/DaHcxDnIkLDSwR6PNNs/Djtg=\",\"1m1ztn/R0T8tqtH0iVFMPF2JB+PmvkDdEdNWjiu3Qug=\",\"FI7Baxp7Q3kY6W+uQyPptLlLDeK7O6eBdWGg7fYw9W8=\"]},\"block_app_hash\":\"JhNlKmsP1MPrOnRR055vnmd+yzZJ1rMR6FN7QCZbRyU=\",\"finality_sig\":\"TrhZZNt69IsHWeXCDS4ylXTUpQHrYn9XuOHEj6pZTG4=\"}"}`
)

func TestParseTestData(t *testing.T) {
	commitBytes, err := base64.StdEncoding.DecodeString("HAzJxMSUIBTMTkzXZv3kzoCKZxuhtRYcf2pN1mSWh7E=")
	require.NoError(t, err)
	prCommit := &types.PubRandCommit{
		StartHeight: 568844,
		NumPubRand:  70000,
		Commitment:  commitBytes,
		EpochNum:    1370,
	}

	// Parse MSG which contains a nested JSON string
	var wrapper struct {
		Msg string `json:"msg"`
	}

	// Parse the inner JSON string into MsgAddFinalitySig
	err = json.Unmarshal([]byte(MSG1), &wrapper)
	require.NoError(t, err)
	var msg1 types.MsgAddFinalitySig
	err = json.Unmarshal([]byte(wrapper.Msg), &msg1)
	require.NoError(t, err)

	// Verify the message
	err = types.VerifyFinalitySig(&msg1, prCommit)
	require.NoError(t, err)

	// Parse MSG2 which also contains a nested JSON string
	var wrapper2 struct {
		Msg string `json:"msg"`
	}
	err = json.Unmarshal([]byte(MSG2), &wrapper2)
	require.NoError(t, err)
	var msg2 types.MsgAddFinalitySig
	err = json.Unmarshal([]byte(wrapper2.Msg), &msg2)
	require.NoError(t, err)

	// Verify the message
	err = types.VerifyFinalitySig(&msg2, prCommit)
	require.NoError(t, err)
}
