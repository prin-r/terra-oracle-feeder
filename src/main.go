package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/terra-project/core/app"

	sdk "github.com/cosmos/cosmos-sdk/types"
	utils "github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	terra_util "github.com/terra-project/core/types/util"

	auth_types "github.com/cosmos/cosmos-sdk/x/auth/types"
	terra_types "github.com/terra-project/core/x/oracle"
)

var (
	TERRA_NODE_URI     = "http://localhost:26657"
	TERRA_REST         = "http://localhost:1317"
	TERRA_KEYBASE_DIR  = "/Users/mumu/.terracli"
	TERRA_KEYNAME      = "q"
	TERRA_KEY_PASSWORD = "12345678"
	TERRA_CHAIN_ID     = "terra-q"
	BAND_REST          = "http://guanyu-devnet.bandchain.org/rest/oracle/request_search?oid=75&calldata=000000044c554e410000000000002710&min_count=4&ask_count=4"
	VALIDATOR_ADDRESS  = "terravaloper1hwjr0j6v5s8cuwtvza9jaqz7s3nfnxyw4r6st6"
	MULTIPLIER         = int64(10000)
	cdc                = app.MakeCodec()
	activeDenoms       = []string{"ukrw", "uusd", "umnt", "usdr"}
)

type BandResponse struct {
	Height int64      `json:"height,string"`
	Result BandResult `json:"result"`
}

type RawRequests struct {
	ExternalID   uint64 `json:"external_id,string"`
	DataSourceID uint64 `json:"data_source_id,string"`
	Calldata     []byte `json:"calldata,string"`
}

type Request struct {
	OracleScriptID      uint64        `json:"oracle_script_id,string"`
	Calldata            []byte        `json:"calldata,string"`
	RequestedValidators []string      `json:"requested_validators"`
	MinCount            uint64        `json:"min_count,string"`
	RequestHeight       uint64        `json:"request_height,string"`
	RequestTime         time.Time     `json:"request_time"`
	ClientID            string        `json:"client_id"`
	RawRequests         []RawRequests `json:"raw_requests"`
}

type RawReports struct {
	ExternalID uint64 `json:"external_id,string"`
	Data       string `json:"data"`
}

type Reports struct {
	Validator       string       `json:"validator"`
	InBeforeResolve bool         `json:"in_before_resolve"`
	RawReports      []RawReports `json:"raw_reports"`
}

type RequestPacketData struct {
	ClientID       string `json:"client_id"`
	OracleScriptID uint64 `json:"oracle_script_id,string"`
	Calldata       []byte `json:"calldata,string"`
	AskCount       uint64 `json:"ask_count,string"`
	MinCount       uint64 `json:"min_count,string"`
}

type ResponsePacketData struct {
	ClientID      string `json:"client_id"`
	RequestID     uint64 `json:"request_id,string"`
	AnsCount      uint64 `json:"ans_count,string"`
	RequestTime   uint64 `json:"request_time,string"`
	ResolveTime   uint64 `json:"resolve_time,string"`
	ResolveStatus uint8  `json:"resolve_status,string"`
	Result        []byte `json:"result,string"`
}

type Packet struct {
	RequestPacketData  RequestPacketData  `json:"RequestPacketData"`
	ResponsePacketData ResponsePacketData `json:"ResponsePacketData"`
}

type BandResult struct {
	Request Request   `json:"request"`
	Reports []Reports `json:"reports"`
	Result  Packet    `json:"result"`
}

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
	terraClient       *client.HTTP
	Params            terra_types.Params
	validator         sdk.ValAddress
	LastPrevoteRound  int64
	LatestBlockHeight int64
	votes             map[string]terra_types.MsgExchangeRateVote
}

func (f *Feeder) fetchParams() {
	res, err := f.terraClient.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryParameters), nil)
	err = cdc.UnmarshalJSON(res.Response.GetValue(), &f.Params)
	if err != nil {
		fmt.Println("Fail to unmarshal Params json", err.Error())
		return
	}
}

func (f *Feeder) commitNewVotes(prices map[string]sdk.Dec) {
	// Salt legnth should be 1~4
	// We use 4 here
	salt, err := generateRandomString(4)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for denom, value := range prices {
		f.votes[denom] = terra_types.NewMsgExchangeRateVote(
			value,
			salt,
			denom,
			sdk.AccAddress(f.validator),
			f.validator,
		)
	}
}

func (f *Feeder) MsgPrevotesFromCurrentCommitVotes() ([]terra_types.MsgExchangeRatePrevote, error) {
	msgs := []terra_types.MsgExchangeRatePrevote{}

	for _, denom := range activeDenoms {
		vote, ok := f.votes[denom]
		if !ok {
			return nil, fmt.Errorf(
				fmt.Sprintf("MsgPrevotesFromCurrentCommitVotes: vote for %s not found in %v", denom, f.votes),
			)
		}

		voteHash, err := terra_types.VoteHash(vote.Salt, vote.ExchangeRate, vote.Denom, f.validator)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}

		msg := terra_types.NewMsgExchangeRatePrevote(
			fmt.Sprintf("%x", voteHash),
			denom,
			sdk.AccAddress(f.validator),
			f.validator,
		)
		msgs = append(msgs, msg)
	}

	return msgs, nil
}

func (f *Feeder) getPrevote() (terra_types.ExchangeRatePrevotes, error) {
	erps := terra_types.ExchangeRatePrevotes{}
	params := terra_types.NewQueryPrevotesParams(f.validator, "")

	bz, err := cdc.MarshalJSON(params)
	if err != nil {
		fmt.Println("Fail to marshal prevote params", err.Error())
		return erps, err
	}

	res, err := f.terraClient.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryPrevotes), bz)

	err = cdc.UnmarshalJSON(res.Response.GetValue(), &erps)
	if err != nil {
		fmt.Println("Fail to unmarshal Params json", err.Error())
		return erps, err
	}

	return erps, nil
}

func (f *Feeder) allVotesAndPrevotesAreCorrespond(prevotes terra_types.ExchangeRatePrevotes) bool {
	for _, pv := range prevotes {
		vote, ok := f.votes[pv.Denom]
		if !ok {
			return false
		}
		if vote.Denom != pv.Denom {
			return false
		}
		voteHash, err := terra_types.VoteHash(vote.Salt, vote.ExchangeRate, vote.Denom, f.validator)
		if err != nil {
			fmt.Println(err.Error())
			return false
		}
		if fmt.Sprintf("%x", voteHash) != pv.Hash {
			return false
		}
	}
	return true
}

func (f *Feeder) broadcast(msgs []sdk.Msg) (*sdk.TxResponse, error) {
	keybase, err := keys.NewKeyBaseFromDir(TERRA_KEYBASE_DIR)
	if err != nil {
		fmt.Println("Fail to create keybase from dir :", err.Error())
		return nil, err
	}

	txBldr := auth_types.NewTxBuilderFromCLI().
		WithTxEncoder(auth_types.DefaultTxEncoder(cdc)).
		WithKeybase(keybase)

	cliCtx := context.NewCLIContext().
		WithCodec(cdc).
		WithClient(f.terraClient).
		WithNodeURI(TERRA_NODE_URI).
		WithTrustNode(true).
		WithFromAddress(sdk.AccAddress(f.validator)).
		WithBroadcastMode("block")

	ptxBldr, err := utils.PrepareTxBuilder(txBldr, cliCtx)
	if err != nil {
		fmt.Println("Fail to prepare tx builder :", err.Error())
		return nil, err
	}

	txBytes, err := ptxBldr.WithChainID(TERRA_CHAIN_ID).BuildAndSign(
		TERRA_KEYNAME,
		TERRA_KEY_PASSWORD,
		msgs,
	)
	if err != nil {
		fmt.Println("Fail to build and sign the transaction :", err.Error())
		return nil, err
	}

	res, err := cliCtx.BroadcastTx(txBytes)
	if err != nil {
		fmt.Println("Fail to broadcast to a Tendermint node :", err.Error())
		return nil, err
	}

	return &res, nil
}

func NewFeeder() Feeder {
	feeder := Feeder{}
	feeder.terraClient = client.NewHTTP(TERRA_NODE_URI, "/websocket")
	valAddress, err := sdk.ValAddressFromBech32(VALIDATOR_ADDRESS)
	if err != nil {
		fmt.Println("Fail to parse validator address", err.Error())
		panic(err)
	}
	feeder.validator = valAddress
	feeder.votes = map[string]terra_types.MsgExchangeRateVote{}
	return feeder
}

func InitSDKConfig() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(terra_util.Bech32PrefixAccAddr, terra_util.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(terra_util.Bech32PrefixValAddr, terra_util.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(terra_util.Bech32PrefixConsAddr, terra_util.Bech32PrefixConsPub)
	config.Seal()
}

func getPriceFromBAND() (map[string]sdk.Dec, error) {
	resp, err := http.Get(BAND_REST)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	br := BandResponse{}
	json.Unmarshal(body, &br)

	res := br.Result.Result.ResponsePacketData.Result

	if len(res) != 32 {
		return nil, fmt.Errorf(fmt.Sprintf("result size should be 32 but got %d", len(res)))
	}

	prices := map[string]sdk.Dec{}
	for i, denom := range activeDenoms {
		prices[denom] = sdk.
			NewDec(int64(binary.BigEndian.Uint64(res[i*8 : (i+1)*8]))).
			Quo(sdk.NewDec(MULTIPLIER))
	}

	return prices, nil
}

func hasPrevotesForAllDenom(prevotes terra_types.ExchangeRatePrevotes) bool {
	prevotesMap := map[string]bool{}
	for _, pv := range prevotes {
		prevotesMap[pv.Denom] = true
	}

	for _, denom := range activeDenoms {
		if !prevotesMap[denom] {
			return false
		}
	}

	return true
}

func main() {

	fmt.Println("Start ...")

	InitSDKConfig()

	feeder := NewFeeder()

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

			status, err := feeder.terraClient.Status()
			if err != nil {
				fmt.Println("Fail to fetch status", err.Error())
				return
			}

			feeder.LatestBlockHeight = status.SyncInfo.LatestBlockHeight
			currentRound := feeder.LatestBlockHeight / feeder.Params.VotePeriod

			fmt.Println(feeder.LatestBlockHeight, currentRound)

			if currentRound > feeder.LastPrevoteRound {
				fmt.Println("get prevotes from terra node")
				prevotes, err := feeder.getPrevote()
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				pass1 := hasPrevotesForAllDenom(prevotes)
				pass2 := feeder.allVotesAndPrevotesAreCorrespond(prevotes)

				if !pass1 {
					fmt.Println("not all prevotes found")
				}
				if !pass2 {
					fmt.Println("there are some votes that do not correspond with the prevotes")
				}

				if pass1 && pass2 {
					fmt.Println("vote for existed prevotes and then create new prevotes")

					msgs := []sdk.Msg{}
					for _, vote := range feeder.votes {
						msgs = append(msgs, vote)
					}

					prices, err := getPriceFromBAND()
					if err != nil {
						fmt.Println(err.Error())
						return
					}

					feeder.commitNewVotes(prices)
					newPrevotes, err := feeder.MsgPrevotesFromCurrentCommitVotes()
					if err != nil {
						fmt.Println(err.Error())
						return
					}
					for _, x := range newPrevotes {
						msgs = append(msgs, x)
					}

					res, err := feeder.broadcast(msgs)
					if err != nil {
						fmt.Println(err.Error())
						return
					}

					fmt.Println("broadcast vote and prevotes : ", res.TxHash, " : ", func() string {
						if res.Code == 0 {
							return "✧･ﾟ:* success *:･ﾟ✧"
						}
						return "fail"
					}())

					feeder.LastPrevoteRound = currentRound

				} else {
					fmt.Println("create new prevotes")

					prices, err := getPriceFromBAND()
					if err != nil {
						fmt.Println(err.Error())
						return
					}

					feeder.commitNewVotes(prices)
					newPrevotes, err := feeder.MsgPrevotesFromCurrentCommitVotes()
					if err != nil {
						fmt.Println(err.Error())
						return
					}

					msgs := []sdk.Msg{}
					for _, x := range newPrevotes {
						msgs = append(msgs, x)
					}

					res, err := feeder.broadcast(msgs)
					if err != nil {
						fmt.Println(err.Error())
						return
					}

					fmt.Println("broadcast prevotes only : ", res.TxHash, " : ", func() string {
						if res.Code == 0 {
							return "✧･ﾟ:* success *:･ﾟ✧"
						}
						return "fail"
					}())

					feeder.LastPrevoteRound = currentRound
				}
			}
		}()
	}
}
