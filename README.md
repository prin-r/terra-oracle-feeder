# Band-Terra-Oracle

Oracle voting script for Terra chain oracle by Band Protocol. This script will periodically ask for block numbers from the chain and then calculate the voting round. If it reaches the voting round, the script will ask Band chain for LUNA's prices so that it can vote for the price relative to various fiat currencies such as USD, KRW, SDR, MNT. Since the vote is a committed review scheme, there will be two main scenarios. The first scenario is that the pre-voting of the previous round is not found in the current round, in which case the script will only commit pre-voting. Another scenario is that the pre-vote of the previous round is found in the current voting round, in which case the script will cast the vote that correspond to the pre-vote and then create a new pre-vote for the current round.

Please keep in mind that this program was only tested with the branch `tags/v0.3.1` of Terra Chain. So please use with Terra Chain branch `tags/v0.3.1`.

## Constant variables

There are 3 types of constants which are Terra constants, Band constants and General constants. Usually, as a Terra validator, you most likely only need to set the `Terra constants` that apply to you.

#### Terra Constants

```go
TERRA_NODE_URI     = "http://localhost:26657"
TERRA_KEYBASE_DIR  = "/Users/mumu/.terracli"
TERRA_KEYNAME      = "q"
TERRA_KEY_PASSWORD = "12345678"
TERRA_CHAIN_ID     = "terra-q"
VALIDATOR_ADDRESS  = "terravaloper1hwjr0j6v5s8cuwtvza9jaqz7s3nfnxyw4r6st6"
```

#### Band Constants

```go
GET_PRICE_TIME_OUT   = 10 * time.Second
MULTIPLIER           = int64(1000000)
LUNA_PRICE_CALLDATA  = LunaPriceCallData{Symbol: "LUNA", Multiplier: MULTIPLIER}
FX_PRICE_CALLDATA    = FxPriceCallData{Symbols: []string{"KRW", "MNT", "XDR"}, Multiplier: MULTIPLIER}
LUNA_PRICE_END_POINT = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=13&calldata=%x&min_count=3&ask_count=4", LUNA_PRICE_CALLDATA.toBytes())
FX_PRICE_END_POINT   = fmt.Sprintf("http://poa-api.bandchain.org/oracle/request_search?oid=9&calldata=%x&min_count=3&ask_count=4", FX_PRICE_CALLDATA.toBytes())
```

#### General Constants

```go
cdc                  = app.MakeCodec()
activeDenoms         = []string{"ukrw", "uusd", "umnt", "usdr"}
```

## Setup

1. setup terra on your local machine [https://github.com/terra-project/core](https://github.com/terra-project/core)
2. switch to branch `tags/v0.3.1`
3. start terra chain
4. as a validator, set these settings using values that are specific to you.

```go
TERRA_NODE_URI     = "http://localhost:26657"
TERRA_KEYBASE_DIR  = "/Users/mumu/.terracli"
TERRA_KEYNAME      = "q"
TERRA_KEY_PASSWORD = "12345678"
TERRA_CHAIN_ID     = "terra-q"
VALIDATOR_ADDRESS  = "terravaloper1hwjr0j6v5s8cuwtvza9jaqz7s3nfnxyw4r6st6"
```

5. run `main.go` after the terra chain has started

```shell=
go run main/main.go
```

## Example Installation On Amazon Lightsail

1. Create a new instance on Amazon Lightsail.

   - I use `Ubuntu 18.04 LTS` with 2GB RAM, 1vCPU, 60GB SSD, 3TB HD

2. Install Golang

```shell=
sudo apt update

wget https://dl.google.com/go/go1.15.1.linux-amd64.tar.gz
sudo tar -xvf go1.15.1.linux-amd64.tar.gz
sudo mv go /usr/local
rm -rf go1.15.1.linux-amd64.tar.gz

export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

source ~/.profile
```

3. Create SSH key for git. So you will be able to `git clone` this repo

```shell=
ssh-keygen -P "" -t rsa -b 4096 -m pem -f key.pem
eval `ssh-agent -s`
ssh-add key.pem
```

4. Add `key.pem.pub` that you just generated to your github

   - [https://docs.github.com/en/enterprise/2.15/user/articles/adding-a-new-ssh-key-to-your-github-account](https://docs.github.com/en/enterprise/2.15/user/articles/adding-a-new-ssh-key-to-your-github-account)

5. Clone this repo

6. Enter the `terra-oracle-feeder`

```shell=
cd terra-oracle-feeder
```

7. Change the setting (I use vim)

![img](https://user-images.githubusercontent.com/12705423/94695170-c9f53980-035f-11eb-98ae-38b9e9240cdc.png)

8. Please note that you should have folder of `terracli`

9. Run

```shell=
go run main/main.go
```

## Dependencies

- [obi](/obi)

## Main Loop Diagram

![img](https://user-images.githubusercontent.com/12705423/94293821-17049480-ff89-11ea-93a3-68eb7ffe4541.png)
