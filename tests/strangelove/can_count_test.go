package strangelove

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v4"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v4/ibc"
	"github.com/strangelove-ventures/interchaintest/v4/testreporter"
	"github.com/strangelove-ventures/interchaintest/v4/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCanCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a new factory and build a local juno and osmosis
	// chain.
	//
	// w/ no gas adjustment, storing contracts fails w/
	// out-of-gas.
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "juno",
			ChainName: "juno1",
			Version:   "latest",
			ChainConfig: ibc.ChainConfig{
				GasPrices:      "0.00ujuno",
				GasAdjustment:  2.0,
				EncodingConfig: wasm.WasmEncoding(),
			},
		},
		{
			Name:      "juno",
			ChainName: "juno2",
			Version:   "latest",
			ChainConfig: ibc.ChainConfig{
				GasPrices:      "0.00ujuno",
				GasAdjustment:  2.0,
				EncodingConfig: wasm.WasmEncoding(),
			},
		},
	})
	chains, err := cf.Chains(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	left, right := chains[0], chains[1]

	client, network := interchaintest.DockerSetup(t)
	relayer := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
	).Build(t, client, network)

	const ibcPath = "juno-juno"
	ic := interchaintest.NewInterchain().
		AddChain(left).
		AddChain(right).
		AddRelayer(relayer, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  left,
			Chain2:  right,
			Relayer: relayer,
			Path:    ibcPath,
		})

	// NopReporter doesn't write to a log file.
	erp := testreporter.NewNopReporter().RelayerExecReporter(t)
	err = ic.Build(ctx, erp, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = relayer.StartRelayer(ctx, erp, ibcPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := relayer.StopRelayer(ctx, erp)
		if err != nil {
			t.Logf("couldn't stop relayer: %s", err)
		}
	})

	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", int64(10_000_000), left, right)
	leftUser := users[0]
	rightUser := users[1]

	leftCosmosChain := left.(*cosmos.CosmosChain)
	rightCosmosChain := right.(*cosmos.CosmosChain)

	codeId, err := leftCosmosChain.StoreContract(ctx, leftUser.KeyName, "../wasms/cw_ibc_example.wasm")
	if err != nil {
		t.Fatal(err)
	}
	leftContract, err := leftCosmosChain.InstantiateContract(ctx, leftUser.KeyName, codeId, "{}", true)
	if err != nil {
		t.Fatal(err)
	}

	codeId, err = rightCosmosChain.StoreContract(ctx, rightUser.KeyName, "../wasms/cw_ibc_example.wasm")
	if err != nil {
		t.Fatal(err)
	}
	rightContract, err := rightCosmosChain.InstantiateContract(ctx, rightUser.KeyName, codeId, "{}", true)
	if err != nil {
		t.Fatal(err)
	}

	leftPort := "wasm." + leftContract
	rightPort := "wasm." + rightContract

	err = relayer.CreateChannel(ctx, erp, ibcPath, ibc.CreateChannelOptions{
		SourcePortName: leftPort,
		DestPortName:   rightPort,
		Order:          ibc.Unordered,
		Version:        "counter-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the channel to get set up
	err = testutil.WaitForBlocks(ctx, 10, left, right)
	require.NoError(t, err)

	channelInfo, err := relayer.GetChannels(ctx, erp, left.Config().ChainID)
	if err != nil {
		t.Fatal(err)
	}
	leftChannel := channelInfo[len(channelInfo)-1].ChannelID
	channelInfo, err = relayer.GetChannels(ctx, erp, right.Config().ChainID)
	if err != nil {
		t.Fatal(err)
	}
	rightChannel := channelInfo[len(channelInfo)-2].ChannelID

	_, err = leftCosmosChain.ExecuteContract(ctx, leftUser.KeyName, leftContract, "{\"increment\": { \"channel\":\""+leftChannel+"\"}}")
	if err != nil {
		t.Fatal(err)
	}

	// wait a couple blocks for the incrementing to relay to the
	// right chain.
	err = testutil.WaitForBlocks(ctx, 10, left, right)
	require.NoError(t, err)

	cmd := []string{right.Config().Bin, "query", "wasm", "contract-state", "all", rightContract,
		"--node", right.GetRPCAddress(),
		"--home", right.HomeDir(),
		"--chain-id", right.Config().ChainID,
		"--output", "json",
	}
	stdout, _, err := right.Exec(ctx, cmd, nil)
	require.NoError(t, err)
	results := &contractStateResp{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)

	t.Log("dumping state")
	for _, kv := range results.Models {
		keyBytes, _ := hex.DecodeString(kv.Key)
		valueBytes, err := base64.StdEncoding.DecodeString(kv.Value)
		require.NoError(t, err)
		t.Logf("------------> %s -> %s", string(keyBytes), string(valueBytes))
	}

	queryMsg := QueryMsg{
		GetCount: &GetCount{Channel: rightChannel},
	}
	var resp QueryResponse
	err = rightCosmosChain.QueryContract(ctx, rightContract, queryMsg, &resp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, uint32(1), resp.Data.Count)
}

type QueryResponse struct {
	Data GetCountQuery `json:"data"`
}

type GetCountQuery struct {
	Count uint32 `json:"count"`
}

type QueryMsg struct {
	GetCount *GetCount `json:"get_count,omitempty"`
}

type GetCount struct {
	Channel string `json:"channel"`
}

type kvPair struct {
	Key   string // hex encoded string
	Value string // b64 encoded json
}

type contractStateResp struct {
	Models []kvPair
}
