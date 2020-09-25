# Band-Terra-Oracle

Oracle voting script for Terra chain oracle by Band Protocol. This script will periodically ask for block numbers from the chain and then calculate the voting round. If it reaches the voting round, the script will ask Band chain for LUNA's prices so that it can vote for the price relative to various fiat currencies such as USD, KRW, SDR, MNT. Since the vote is a committed review scheme, there will be two main scenarios. The first scenario is that the pre-voting of the previous round is not found in the current round, in which case the script will only commit pre-voting. Another scenario is that the pre-vote of the previous round is found in the current voting round, in which case the script will cast the vote that correspond to the pre-vote and then create a new pre-vote for the current round.

## Setup

1. setup terra on your local machine [https://github.com/terra-project/core](https://github.com/terra-project/core)
2. switch to branch `tags/v0.3.1`
3. start terra chain

## Run

1. run `main.go` after the terra chain has started

```sh
go run main/main.go
```

## Constant variables

```go
TERRA_NODE_URI       = "http://localhost:26657"
TERRA_REST           = "http://localhost:1317"
TERRA_KEYBASE_DIR    = "/Users/mumu/.terracli"
TERRA_KEYNAME        = "q"
TERRA_KEY_PASSWORD   = "12345678"
TERRA_CHAIN_ID       = "terra-q"
GET_PRICE_TIME_OUT   = 10 * time.Second
MULTIPLIER           = int64(1000000)
LUNA_PRICE_CALLDATA  = LunaPriceCallData{Symbol: "LUNA", Multiplier: MULTIPLIER}
FX_PRICE_CALLDATA    = FxPriceCallData{Symbols: []string{"KRW", "MNT", "XDR"}, Multiplier: MULTIPLIER}
LUNA_PRICE_END_POINT = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=13&calldata=%x&min_count=3&ask_count=4", LUNA_PRICE_CALLDATA.toBytes())
FX_PRICE_END_POINT   = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=9&calldata=%x&min_count=3&ask_count=4", FX_PRICE_CALLDATA.toBytes())
VALIDATOR_ADDRESS    = "terravaloper1hwjr0j6v5s8cuwtvza9jaqz7s3nfnxyw4r6st6"
cdc                  = app.MakeCodec()
activeDenoms         = []string{"ukrw", "uusd", "umnt", "usdr"}
```

## Obi Library

- [obi](/obi)

## Main Loop Diagram

![img](https://user-images.githubusercontent.com/12705423/94293821-17049480-ff89-11ea-93a3-68eb7ffe4541.png)
