package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	client "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/terra-project/core/app"

	sdk "github.com/cosmos/cosmos-sdk/types"
	utils "github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	terra_util "github.com/terra-project/core/types/util"

	auth_types "github.com/cosmos/cosmos-sdk/x/auth/types"
	terra_types "github.com/terra-project/core/x/oracle"

	obi "github.com/bandprotocol/band-terra-oracle/obi"
)

// Terra constants
var (
	TERRA_NODE_URI     = "http://localhost:26657"
	TERRA_KEYBASE_DIR  = "/Users/mumu/.terracli"
	TERRA_KEYNAME      = "q"
	TERRA_KEY_PASSWORD = "12345678"
	TERRA_CHAIN_ID     = "terra-q"
	VALIDATOR_ADDRESS  = "terravaloper1q5fz9h24t856tzj52pjymgqk2acay6fxejfrsz"
)

// Band constants
var (
	GET_PRICE_TIME_OUT   = 20 * time.Second
	MULTIPLIER           = int64(1000000)
	LUNA_PRICE_CALLDATA  = LunaPriceCallData{Symbol: "LUNA", Multiplier: MULTIPLIER}
	FX_PRICE_CALLDATA    = FxPriceCallData{Symbols: []string{"KRW", "MNT", "XDR"}, Multiplier: MULTIPLIER}
	LUNA_PRICE_END_POINT = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=13&calldata=%x&min_count=3&ask_count=4", LUNA_PRICE_CALLDATA.toBytes())
	FX_PRICE_END_POINT   = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=9&calldata=%x&min_count=3&ask_count=4", FX_PRICE_CALLDATA.toBytes())
)

// General constants
var (
	cdc          = app.MakeCodec()
	activeDenoms = []string{"ukrw", "uusd", "umnt", "usdr"}
)

type LunaPriceCallData struct {
	Symbol     string
	Multiplier int64
}

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
	ResolveStatus uint8  `json:"resolve_status"`
	Result        []byte `json:"result,string"`
}

type Packet struct {
	RequestPacketData  RequestPacketData  `json:"request_packet_data"`
	ResponsePacketData ResponsePacketData `json:"response_packet_data"`
}

type BandResult struct {
	Request Request   `json:"request"`
	Reports []Reports `json:"reports"`
	Result  Packet    `json:"result"`
}

type LunaPrice struct {
	CryptoCompareUSD int64
	CoinGeckoUSD     int64
	HuobiproUSD      int64
	BittrexUSD       int64
	BithumbKRW       int64
	CoinoneKRW       int64
	CoinmarketcapUSD int64
}

type FxPriceUSD []uint64

type Feeder struct {
	terraClient       *client.HTTP
	Params            terra_types.Params
	validator         sdk.ValAddress
	LastPrevoteRound  int64
	LatestBlockHeight int64
	votes             map[string]terra_types.MsgExchangeRateVote
}

func (cd *LunaPriceCallData) toBytes() []byte {
	b, err := obi.Encode(*cd)
	if err != nil {
		panic(err)
	}
	return b
}

type FxPriceCallData struct {
	Symbols    []string
	Multiplier int64
}

func (cd *FxPriceCallData) toBytes() []byte {
	b, err := obi.Encode(*cd)
	if err != nil {
		panic(err)
	}
	return b
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

func logError(err error) {
	pc, fn, line, _ := runtime.Caller(1)
	log.Printf("‼️ %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), fn, line, err)
}

func printStatus(head string, status string, res *sdk.TxResponse) {
	tail := "✧･ﾟ✨:* success *:✨･ﾟ✧"
	if res.Code != 0 {
		head = "🚫"
		tail = "fail ⁉️"
	}
	fmt.Printf("%s %s : %s : %s \n", head, status, res.TxHash, tail)
}

func decsPretty(decs []sdk.Dec) string {
	tmp := []string{}
	for _, dec := range decs {
		s := dec.String()
		if strings.Index(s, ".")+6 <= len(s) {
			tmp = append(tmp, s[:strings.Index(s, ".")+6])
		} else {
			tmp = append(tmp, s)
		}
	}
	return fmt.Sprintf("%v", tmp)
}

func (f *Feeder) fetchParams() {
	res, err := f.terraClient.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryParameters), nil)
	if err != nil {
		logError(err)
		return
	}

	err = cdc.UnmarshalJSON(res.Response.GetValue(), &f.Params)
	if err != nil {
		logError(fmt.Errorf("Fail to unmarshal Params json: %v", err))
		return
	}
}

func (f *Feeder) commitNewVotes(prices map[string]sdk.Dec) {
	// Salt legnth should be 1~4
	// We use 4 here
	salt, err := generateRandomString(4)
	if err != nil {
		fmt.Println(err.Error(), 205)
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

		voteHash := terra_types.GetVoteHash(vote.Salt, vote.ExchangeRate, vote.Denom, f.validator)
		msg := terra_types.NewMsgExchangeRatePrevote(
			voteHash,
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
		logError(fmt.Errorf("Fail to marshal prevote params: %v", err))
		return erps, err
	}

	res, err := f.terraClient.ABCIQuery(fmt.Sprintf("custom/%s/%s", terra_types.QuerierRoute, terra_types.QueryPrevotes), bz)

	err = cdc.UnmarshalJSON(res.Response.GetValue(), &erps)
	if err != nil {
		logError(fmt.Errorf("Fail to unmarshal Params json: %v", err))
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
		voteHash := terra_types.GetVoteHash(vote.Salt, vote.ExchangeRate, vote.Denom, f.validator)

		if voteHash.Equal(pv.Hash) {
			return false
		}
	}
	return true
}

func (f *Feeder) broadcast(msgs []sdk.Msg) (*sdk.TxResponse, error) {
	keybase, err := keys.NewKeyring("terra", "test", TERRA_KEYBASE_DIR, nil)
	if err != nil {
		logError(fmt.Errorf("Fail to create keybase from dir: %v", err))
		return nil, err
	}

	txBldr := auth_types.NewTxBuilder(
		auth_types.DefaultTxEncoder(cdc),
		0, 0, 200000, 0.0, false, TERRA_CHAIN_ID, "",
		sdk.NewCoins(sdk.NewCoin("uluna", sdk.NewInt(0))),
		sdk.NewDecCoins(sdk.NewDecCoin("uluna", sdk.NewInt(0))),
	).WithKeybase(keybase)

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
	valAddress, err := sdk.ValAddressFromBech32(VALIDATOR_ADDRESS)
	if err != nil {
		fmt.Println("Fail to parse validator address", err.Error())
		panic(err)
	}
	feeder := Feeder{}
	feeder.terraClient, err = client.New(TERRA_NODE_URI, "/websocket")
	if err != nil {
		fmt.Println("Fail to create http client", err.Error())
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

func getLUNAPriceFromDataSources() (LunaPrice, error) {
	resp, err := http.Get(LUNA_PRICE_END_POINT)
	if err != nil {
		return LunaPrice{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return LunaPrice{}, err
	}

	br := BandResponse{}
	err = json.Unmarshal(body, &br)
	if err != nil {
		return LunaPrice{}, fmt.Errorf("fail to unmarshal luna price from ds, %v, %s", err, string(body[:]))
	}

	var lp LunaPrice
	obi.Decode(br.Result.Result.ResponsePacketData.Result, &lp)

	return lp, nil
}

func getStandardCurrencyPrices() (FxPriceUSD, error) {
	resp, err := http.Get(FX_PRICE_END_POINT)
	if err != nil {
		return FxPriceUSD{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return FxPriceUSD{}, err
	}

	br := BandResponse{}
	err = json.Unmarshal(body, &br)
	if err != nil {
		return FxPriceUSD{}, fmt.Errorf("fail to unmarshal fx price from ds, %v, %s", err, string(body[:]))
	}

	var fpu FxPriceUSD
	obi.Decode(br.Result.Result.ResponsePacketData.Result, &fpu)

	return fpu, nil
}

func medianDec(decs []sdk.Dec) sdk.Dec {
	sort.Slice(decs, func(i, j int) bool {
		return decs[i].LT(decs[j])
	})
	return decs[len(decs)/2]
}

func getLUNAPrices() (map[string]sdk.Dec, error) {
	type priceWithErr struct {
		Val interface{}
		Err error
	}

	ch := make(chan priceWithErr, 2)

	go func() {
		lp, err := getLUNAPriceFromDataSources()
		ch <- priceWithErr{Val: lp, Err: err}
	}()
	go func() {
		fpu, err := getStandardCurrencyPrices()
		ch <- priceWithErr{Val: fpu, Err: err}
	}()

	priceWithErrList := []priceWithErr{}
	start := time.Now()
	for len(priceWithErrList) < 2 {
		x, ok := <-ch
		if ok {
			priceWithErrList = append(priceWithErrList, x)
		}
		if time.Since(start) >= GET_PRICE_TIME_OUT {
			return nil, fmt.Errorf("⏰ getting price has timeout")
		}
	}

	var lp LunaPrice
	var fpu FxPriceUSD
	for _, pwe := range priceWithErrList {
		if pwe.Err != nil {
			return nil, pwe.Err
		}
		switch pwe.Val.(type) {
		case LunaPrice:
			lp = pwe.Val.(LunaPrice)
		case FxPriceUSD:
			fpu = pwe.Val.(FxPriceUSD)
		default:
			return nil, fmt.Errorf("unknown type %v", pwe.Val)
		}
	}

	multiplier := sdk.NewDec(MULTIPLIER)

	fmt.Printf("🌕 luna prices: %v \n", lp)
	fmt.Printf("💵 fx prices: %v \n", fpu)

	krws := []sdk.Dec{}
	usds := []sdk.Dec{}
	if lp.BithumbKRW >= 0 {
		krws = append(krws, sdk.NewDec(lp.BithumbKRW).Quo(multiplier))
		usds = append(usds, sdk.NewDec(lp.BithumbKRW).Mul(sdk.NewDec(int64(fpu[0]))).Quo(multiplier))
	}
	if lp.CoinoneKRW >= 0 {
		krws = append(krws, sdk.NewDec(lp.CoinoneKRW).Quo(multiplier))
		usds = append(usds, sdk.NewDec(lp.CoinoneKRW).Mul(sdk.NewDec(int64(fpu[0]))).Quo(multiplier))
	}
	if lp.BittrexUSD >= 0 {
		krws = append(krws, sdk.NewDec(lp.BittrexUSD).Quo(sdk.NewDec(int64(fpu[0]))))
		usds = append(usds, sdk.NewDec(lp.BittrexUSD))
	}
	if lp.CoinGeckoUSD >= 0 {
		krws = append(krws, sdk.NewDec(lp.CoinGeckoUSD).Quo(sdk.NewDec(int64(fpu[0]))))
		usds = append(usds, sdk.NewDec(lp.CoinGeckoUSD))
	}
	if lp.CryptoCompareUSD >= 0 {
		krws = append(krws, sdk.NewDec(lp.CryptoCompareUSD).Quo(sdk.NewDec(int64(fpu[0]))))
		usds = append(usds, sdk.NewDec(lp.CryptoCompareUSD))
	}
	if lp.HuobiproUSD >= 0 {
		krws = append(krws, sdk.NewDec(lp.HuobiproUSD).Quo(sdk.NewDec(int64(fpu[0]))))
		usds = append(usds, sdk.NewDec(lp.HuobiproUSD))
	}
	if lp.CoinmarketcapUSD >= 0 {
		krws = append(krws, sdk.NewDec(lp.CoinmarketcapUSD).Quo(sdk.NewDec(int64(fpu[0]))))
		usds = append(usds, sdk.NewDec(lp.CoinmarketcapUSD))
	}

	fmt.Printf("krw rates: %s \n", decsPretty(krws))
	fmt.Printf("usds rates: %s \n", decsPretty(func() []sdk.Dec {
		tmp := []sdk.Dec{}
		for _, x := range usds {
			tmp = append(tmp, x.Quo(multiplier))
		}
		return tmp
	}()))

	if len(krws) == 0 || len(usds) == 0 {
		return nil, fmt.Errorf("‼️🔥 fail to get luna price from every sources 🔥‼️")
	}

	medKRW := medianDec(krws)
	medUSD := medianDec(usds)

	result := map[string]sdk.Dec{
		"ukrw": medKRW,
		"uusd": medUSD.Quo(multiplier),
		"umnt": medUSD.Quo(sdk.NewDec(int64(fpu[1]))),
		"usdr": medUSD.Quo(sdk.NewDec(int64(fpu[2]))),
	}

	fmt.Printf("🌟 result: %v \n", result)

	return result, nil
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
					logError(fmt.Errorf("Unknown error: %v", r))
				}

				time.Sleep(1 * time.Second)
			}()

			status, err := feeder.terraClient.Status()
			if err != nil {
				logError(fmt.Errorf("Fail to fetch status %v", err))
				return
			}

			feeder.LatestBlockHeight = status.SyncInfo.LatestBlockHeight
			currentRound := feeder.LatestBlockHeight / feeder.Params.VotePeriod

			fmt.Printf("\rOn latestBlockHeight=%d currentRound=%d", feeder.LatestBlockHeight, currentRound)

			if currentRound > feeder.LastPrevoteRound {
				fmt.Println("get prevotes from terra node")
				prevotes, err := feeder.getPrevote()
				if err != nil {
					logError(err)
					return
				}

				prices, err := getLUNAPrices()
				if err != nil {
					logError(err)
					return
				}

				msgs := []sdk.Msg{}

				pass1 := hasPrevotesForAllDenom(prevotes)
				pass2 := feeder.allVotesAndPrevotesAreCorrespond(prevotes)

				if !pass1 {
					fmt.Println("🔍 not all prevotes found")
				}
				if !pass2 {
					fmt.Println("🧂 there are some votes that do not correspond with the prevotes")
				}

				if pass1 && pass2 {
					fmt.Println("🗳️ vote for existed prevotes and then create new prevotes")

					for _, vote := range feeder.votes {
						msgs = append(msgs, vote)
					}
				} else {
					fmt.Println("create new prevotes")
				}

				feeder.commitNewVotes(prices)
				newPrevotes, err := feeder.MsgPrevotesFromCurrentCommitVotes()
				if err != nil {
					logError(err)
					return
				}

				for _, x := range newPrevotes {
					msgs = append(msgs, x)
				}

				res, err := feeder.broadcast(msgs)
				if err != nil {
					logError(err)
					return
				}

				if pass1 && pass2 {
					printStatus("🍻", "broadcast vote and prevotes", res)
				} else {
					printStatus("🍺", "broadcast prevotes only", res)
				}

				feeder.LastPrevoteRound = currentRound
			}
		}()
	}
}
