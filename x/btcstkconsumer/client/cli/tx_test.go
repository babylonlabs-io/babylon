package cli_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	testutilcli "github.com/babylonlabs-io/babylon/v4/testutil/cli"
	bsccli "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/client/cli"
)

func TestNewRegisterConsumerCmd(t *testing.T) {
	t.Parallel()
	clientCtx, addrs := testutilcli.SetupClientContext(t)
	cmd := bsccli.NewRegisterConsumerCmd()

	testCases := []struct {
		name         string
		args         []string
		expectErrMsg string
	}{
		{
			"missing consumer-id",
			[]string{
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"requires at least 4 arg(s), only received 0",
		},
		{
			"missing consumer-name",
			[]string{
				"test-consumer-id",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"requires at least 4 arg(s), only received 1",
		},
		{
			"missing consumer-description",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"requires at least 4 arg(s), only received 2",
		},
		{
			"missing babylon-commission",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"requires at least 4 arg(s), only received 3",
		},
		{
			"empty consumer-id",
			[]string{
				"",
				"Test Consumer",
				"Test Description",
				"0.1",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"consumer's id cannot be empty",
		},
		{
			"empty consumer-name",
			[]string{
				"test-consumer-id",
				"",
				"Test Description",
				"0.1",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"consumer's name cannot be empty",
		},
		{
			"empty consumer-description",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"",
				"0.1",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"consumer's description cannot be empty",
		},
		{
			"empty babylon-commission",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"babylon rewards commission cannot be empty",
		},
		{
			"invalid babylon-commission format",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"not-a-number",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"invalid babylon rewards commission",
		},
		{
			"babylon-commission greater than 1",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"1.5",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"babylon commission cannot be greater than 1.0",
		},
		{
			"valid cosmos consumer registration (min commission 0.01)",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"0.01",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
		{
			"valid cosmos consumer registration (commission 0.1)",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"0.1",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
		{
			"valid cosmos consumer registration (commission 1.0)",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"1.0",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
		{
			"valid rollup consumer registration",
			[]string{
				"test-rollup-id",
				"Test Rollup",
				"Test Rollup Description",
				"0.05",
				"babylon1abc123def456ghi789jkl012mno345pqr678stu",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
		{
			"valid consumer registration with decimal commission",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"0.025",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
		{
			"valid consumer registration with high precision commission",
			[]string{
				"test-consumer-id",
				"Test Consumer",
				"Test Description",
				"0.123456789012345678",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, addrs[0]),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)).String()),
			},
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := testutilcli.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
				return
			}
			require.NoError(t, err, "test: %s\noutput: %s", tc.name, out.String())
			err = clientCtx.Codec.UnmarshalJSON(out.Bytes(), &sdk.TxResponse{})
			require.NoError(t, err, out.String(), "test: %s, output\n:", tc.name, out.String())
		})
	}
}

func TestRegisterConsumerCmdUsage(t *testing.T) {
	t.Parallel()
	cmd := bsccli.NewRegisterConsumerCmd()

	require.Contains(t, cmd.Use, "register-consumer")
	require.Contains(t, cmd.Use, "<consumer-id>")
	require.Contains(t, cmd.Use, "<name>")
	require.Contains(t, cmd.Use, "<description>")
	require.Contains(t, cmd.Use, "<babylon-rewards-commission>")
	require.Contains(t, cmd.Use, "[rollup-address]")
	require.Equal(t, "Registers a consumer", cmd.Short)
	require.Contains(t, cmd.Long, "babylon-rewards-commission is the commission rate")
	require.Contains(t, cmd.Long, "between 0 and 1")
}
