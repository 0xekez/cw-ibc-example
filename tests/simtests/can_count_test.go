package simtests

import (
	"testing"

	"github.com/CosmWasm/wasmd/x/wasm/ibctesting"
	"github.com/stretchr/testify/require"

	sdkibctesting "github.com/cosmos/ibc-go/v4/testing"
)

func TestIBCCounting(t *testing.T) {
	// creates two simulated chains and returns a type for working
	// with them. a third, optional argument is a list of options
	// for each chain's configuration. some options can be found
	// in `wasmd/x/wasm/keeper/options.go`.
	c := ibctesting.NewCoordinator(t, 2)
	chainA := c.GetChain(sdkibctesting.GetChainID(0))
	chainB := c.GetChain(sdkibctesting.GetChainID(1))

	// see `../../justfile` for how this is placed here.
	//
	// codeIDs are sequental so the contract will have code ID one
	// on both chains.
	chainA.StoreCodeFile("../wasms/cw_ibc_example.wasm")
	chainB.StoreCodeFile("../wasms/cw_ibc_example.wasm")

	// instantiate a cw_ibc_example contract on both chains and
	// get its IBC port.
	ac := Instantiate(t, chainA, 1)
	bc := Instantiate(t, chainB, 1)
	aPort := chainA.ContractInfo(ac).IBCPortID
	bPort := chainB.ContractInfo(bc).IBCPortID

	// make an ibc connection between the two contracts.
	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig = ChannelConfig(aPort)
	path.EndpointB.ChannelConfig = ChannelConfig(bPort)
	c.Setup(path)

	// create an account on each chain to execute messages with.
	a := GenAccount(t, chainA)
	b := GenAccount(t, chainB)

	// Check that incrementing works.
	zero, err := a.ExecuteIncrement(t, &ac, path.EndpointA.ChannelID)
	require.NoError(t, err)
	require.Equal(t, uint32(0), zero)
	require.NoError(t, c.RelayAndAckPendingPackets(path))

	one, err := b.ExecuteIncrement(t, &bc, path.EndpointB.ChannelID)
	require.NoError(t, err)
	require.Equal(t, uint32(1), one)
	require.NoError(t, c.RelayAndAckPendingPackets(path))

	one, err = a.ExecuteIncrement(t, &ac, path.EndpointA.ChannelID)
	require.NoError(t, err)
	require.Equal(t, uint32(1), one)
	require.NoError(t, c.RelayAndAckPendingPackets(path))

	two, err := b.ExecuteIncrement(t, &bc, path.EndpointB.ChannelID)
	require.NoError(t, err)
	require.Equal(t, uint32(2), two)
	require.NoError(t, c.RelayAndAckPendingPackets(path))
}
