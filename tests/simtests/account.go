package simtests

import (
	"testing"

	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm/ibctesting"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"
)

type Account struct {
	PrivKey cryptotypes.PrivKey
	PubKey  cryptotypes.PubKey
	Address sdk.AccAddress
	Acc     authtypes.AccountI
	Chain   *ibctesting.TestChain // lfg garbage collection!!
}

// Generates a new account on the provided chain with 100_000_000
// tokens of the chain's bonding denom.
func GenAccount(t *testing.T, chain *ibctesting.TestChain) Account {
	privkey := secp256k1.GenPrivKey()
	pubkey := privkey.PubKey()
	addr := sdk.AccAddress(pubkey.Address())

	bondDenom := chain.App.StakingKeeper.BondDenom(chain.GetContext())
	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, sdk.NewInt(100000000)))

	err := chain.App.BankKeeper.MintCoins(chain.GetContext(), minttypes.ModuleName, coins)
	require.NoError(t, err)

	err = chain.App.BankKeeper.SendCoinsFromModuleToAccount(chain.GetContext(), minttypes.ModuleName, addr, coins)
	require.NoError(t, err)

	accountNumber := chain.App.AccountKeeper.GetNextAccountNumber(chain.GetContext())
	baseAcc := authtypes.NewBaseAccount(addr, pubkey, accountNumber, 0)
	chain.App.AccountKeeper.SetAccount(chain.GetContext(), baseAcc)

	return Account{
		PrivKey: privkey,
		PubKey:  pubkey,
		Address: addr,
		Acc:     baseAcc,
		Chain:   chain,
	}
}

// Sends some messages from an account.
func (a *Account) Send(t *testing.T, msgs ...sdk.Msg) (*sdk.Result, error) {
	a.Chain.Coordinator.UpdateTime()

	_, r, err := app.SignAndDeliver(
		t,
		a.Chain.TxConfig,
		a.Chain.App.BaseApp,
		a.Chain.GetContext().BlockHeader(),
		msgs,
		a.Chain.ChainID,
		[]uint64{a.Acc.GetAccountNumber()},
		[]uint64{a.Acc.GetSequence()},
		a.PrivKey,
	)
	if err != nil {
		t.Log("goodbye")
		return r, err
	}

	a.Chain.NextBlock()

	// increment sequence for successful transaction execution
	err = a.Acc.SetSequence(a.Acc.GetSequence() + 1)
	if err != nil {
		return nil, err
	}

	a.Chain.Coordinator.IncrementTime()

	a.Chain.CaptureIBCEvents(r)

	return r, nil
}
