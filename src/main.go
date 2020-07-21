package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/terra-project/core/app"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/terra-project/core/types/util"

	terra_types "github.com/terra-project/core/x/oracle"
)

var (
	TERRA_NODE_URI    = "http://localhost:26657"
	TERRA_REST        = "http://localhost:1317"
	VALIDATOR_ADDRESS = "terravaloper1hwjr0j6v5s8cuwtvza9jaqz7s3nfnxyw4r6st6"
	cdc               = app.MakeCodec()
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes, err := generateRandomBytes(n)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes), nil
}

type Feeder struct {
	client            *client.HTTP
	Params            terra_types.Params
	LastPrevoteRound  int64
	LatestBlockHeight int64
}

func (f *Feeder) fetchParams() {
	res, err := f.client.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryParameters), nil)
	err = cdc.UnmarshalJSON(res.Response.GetValue(), &f.Params)
	if err != nil {
		fmt.Println("Fail to unmarshal Params json", err.Error())
		return
	}
}

func (f *Feeder) getPrevote() (terra_types.ExchangeRatePrevotes, error) {
	erps := terra_types.ExchangeRatePrevotes{}
	valAddress, err := sdk.ValAddressFromBech32(VALIDATOR_ADDRESS)
	if err != nil {
		fmt.Println("Fail to parse validator address", err.Error())
		return erps, err
	}

	params := terra_types.NewQueryPrevotesParams(valAddress, "")

	bz, err := cdc.MarshalJSON(params)
	if err != nil {
		fmt.Println("Fail to marshal prevote params", err.Error())
		return erps, err
	}

	res, err := f.client.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryPrevotes), bz)

	err = cdc.UnmarshalJSON(res.Response.GetValue(), &erps)
	if err != nil {
		fmt.Println("Fail to unmarshal Params json", err.Error())
		return erps, err
	}

	return erps, nil
}

func main() {

	fmt.Println("Start ...")

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(util.Bech32PrefixAccAddr, util.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(util.Bech32PrefixValAddr, util.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(util.Bech32PrefixConsAddr, util.Bech32PrefixConsPub)
	config.Seal()

	feeder := Feeder{}
	feeder.client = client.NewHTTP(TERRA_NODE_URI, "/websocket")

	for feeder.Params.VotePeriod == 0 {
		feeder.fetchParams()
		time.Sleep(1 * time.Second)
	}

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Unknown error", r)
				}

				time.Sleep(1 * time.Second)
			}()

			status, err := feeder.client.Status()
			if err != nil {
				fmt.Println("Fail to fetch status", err.Error())
				return
			}

			feeder.LatestBlockHeight = status.SyncInfo.LatestBlockHeight
			nextRound := feeder.LatestBlockHeight / feeder.Params.VotePeriod

			fmt.Println(feeder.LatestBlockHeight, nextRound)

			if nextRound > feeder.LastPrevoteRound {
				erps, err := feeder.getPrevote()
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				salt, err := generateRandomString(10)
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				fmt.Println(erps)
				fmt.Println(salt)
			}
		}()
	}

	fmt.Println(feeder)

}
