package strangelove

import (
	"context"
	"testing"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	"github.com/strangelove-ventures/interchaintest/v4"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v4/ibc"
	"github.com/strangelove-ventures/interchaintest/v4/testreporter"
	"github.com/strangelove-ventures/interchaintest/v4/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// The default timeout of packets sent by the cw-ibc-example contract.
const DEFAULT_TIMEOUT = time.Minute * 2

// Sets up two chains, creates a connection between them with a very
// small trusting period, creates a channel between two cw-ibc-example
// contracts on that channel, causes the channel to expire.
func TestLightClientExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
	trustingPeriod := time.Duration(time.Minute * 1)

	ic := interchaintest.NewInterchain().
		AddChain(left).
		AddChain(right).
		AddRelayer(relayer, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  left,
			Chain2:  right,
			Relayer: relayer,
			Path:    ibcPath,
			CreateClientOpts: ibc.CreateClientOptions{
				TrustingPeriod: trustingPeriod.String(),
			},
		})

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
	// TODO: this fails intermitently with index -1 is out of
	// bounds. Seems like there is a race between the token
	// transfer and our channel.
	rightChannel := channelInfo[len(channelInfo)-2].ChannelID // two channels by default: ours and ics-20-transfer

	_, err = leftCosmosChain.ExecuteContract(ctx, leftUser.KeyName, leftContract, "{\"increment\": { \"channel\":\""+leftChannel+"\"}}")
	if err != nil {
		t.Fatal(err)
	}

	// wait a couple blocks for the incrementing to relay to the
	// right chain.
	err = testutil.WaitForBlocks(ctx, 10, left, right)
	require.NoError(t, err)

	queryMsg := QueryMsg{
		GetCount: &GetCount{Channel: rightChannel},
	}
	var resp QueryResponse
	err = rightCosmosChain.QueryContract(ctx, rightContract, queryMsg, &resp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, uint32(1), resp.Data.Count)

	// Stop the relayer for the trusting period. This should cause
	// the channel to expire as the light client should time out.
	relayer.StopRelayer(ctx, erp)
	time.Sleep(trustingPeriod)
	relayer.UpdateClients(ctx, erp, ibcPath)

	// First, we check that the client is in the expired state.
	grpcConn, err := grpc.Dial(
		leftCosmosChain.GetHostGRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := grpcConn.Close(); err != nil {
			t.Logf("closing GRPC: %s", err)
		}
	})

	clientClient := clienttypes.NewQueryClient(grpcConn)
	statusResp, err := clientClient.ClientStatus(ctx, &clienttypes.QueryClientStatusRequest{
		ClientId: "07-tendermint-0",
	})
	if err != nil {
		t.Fatal("querying client state:", err)
	}
	require.Equal(t, "Expired", statusResp.Status)

	// Having confirmed that the client is expired, we now check
	// the connection. Interestingly, the connection remains open
	// even if the client is expired!
	queryMsg = QueryMsg{
		GetCount: &GetCount{Channel: rightChannel},
	}
	err = rightCosmosChain.QueryContract(ctx, rightContract, queryMsg, &resp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, uint32(1), resp.Data.Count)

	connections, err := relayer.GetConnections(ctx, erp, left.Config().ChainID)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "STATE_OPEN", connections[0].State)

	// Now, we restart the relayer and attempt to send a packet
	// over the connection with the expired client.
	relayer.StartRelayer(ctx, erp, ibcPath)
	_, err = leftCosmosChain.ExecuteContract(ctx, leftUser.KeyName, leftContract, "{\"increment\": { \"channel\":\""+leftChannel+"\"}}")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the timeout duration.
	time.Sleep(DEFAULT_TIMEOUT)
	// give the relayer some time to relay the timeout back (if it
	// were to be sendable).
	err = testutil.WaitForBlocks(ctx, 10, left, right)
	require.NoError(t, err)

	err = rightCosmosChain.QueryContract(ctx, rightContract, queryMsg, &resp)
	if err != nil {
		t.Fatal(err)
	}
	// We don't expect the packet to get delivered as the client
	// is expired, so the count should still be one.
	require.Equal(t, uint32(1), resp.Data.Count)

	timeoutQuery := QueryMsg{
		GetTimeoutCount: &GetCount{Channel: leftChannel},
	}
	var timeoutResp QueryResponse
	err = leftCosmosChain.QueryContract(ctx, leftContract, timeoutQuery, &resp)
	if err != nil {
		t.Fatal(err)
	}
	// As we've waited for the timeout duration, and then an
	// additional ten blocks, the timeout still not being
	// delivered seems like strong evidence that timeouts do not
	// get delivered for expired clients.
	require.Equal(t, uint32(0), timeoutResp.Data.Count)
}
