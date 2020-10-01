module github.com/bandprotocol/band-terra-oracle

go 1.23

require (
	github.com/cosmos/cosmos-sdk v0.39.1
	github.com/tendermint/tendermint v0.33.7
	github.com/terra-project/core v0.4.0
)

replace github.com/CosmWasm/go-cosmwasm => github.com/terra-project/go-cosmwasm v0.10.1-terra
