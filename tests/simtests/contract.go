package simtests

import (
	"encoding/json"
	"testing"

	"github.com/CosmWasm/wasmd/x/wasm/ibctesting"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	sdkibctesting "github.com/cosmos/ibc-go/v4/testing"
)

type InstantiateMsg struct {
}

type ExecuteMsg struct {
	Increment *Increment `json:"increment,omitempty"`
}

type Increment struct {
	Channel string `json:"channel"`
}

type QueryMsg struct {
	GetCount *GetCount `json:"get_count,omitempty"`
}

type GetCount struct {
	Channel string `json:"channel"`
}

type GetCountResponse struct {
	Count uint32 `json:"count"`
}

// Calls the increment method and returns the current value.
func (a *Account) ExecuteIncrement(t *testing.T, contract *sdk.AccAddress, channel string) (uint32, error) {
	_, err := a.Send(t, a.WasmExecute(
		contract,
		ExecuteMsg{
			Increment: &Increment{Channel: channel},
		},
	))
	if err != nil {
		return 0, err
	}
	query, err := json.Marshal(QueryMsg{
		GetCount: &GetCount{Channel: channel},
	})
	if err != nil {
		return 0, err
	}
	res, err := a.Chain.App.WasmKeeper.QuerySmart(a.Chain.GetContext(), *contract, query)
	if err != nil {
		return 0, err
	}
	var count GetCountResponse
	json.Unmarshal(res, &count)
	return count.Count, nil
}

func Instantiate(t *testing.T, chain *ibctesting.TestChain, codeId uint64) sdk.AccAddress {
	instantiate, err := json.Marshal(InstantiateMsg{})
	if err != nil {
		t.Fatal(err)
	}
	return chain.InstantiateContract(codeId, instantiate)
}

func ChannelConfig(port string) *sdkibctesting.ChannelConfig {
	return &sdkibctesting.ChannelConfig{
		PortID:  port,
		Version: "counter-1",
		Order:   channeltypes.UNORDERED,
	}
}
