# solpipe-tool

This go module allows staked-valditors to sell bandwidth and developers to buy bandwidth via the onchain Solpipe program.  The Solpipe Solana program is a continuous bandwidth auctioning mechanism.


Bandwidth is a consumable commodity.  Developers no longer have direct access to the lead validator without a staked validator.  Access to the lead validator is over Quic protocol with QoS (quality of service) rate limiting by staked SOL amount.  Thus, developers must route writes through staked validators to write to the chain.

Staked validators need to allocate scare routing capacity.  Go-Staker is the Golang interface to the Solpipe program.

## Vocuabulary

| *Term* | *Definition* |
|------------|---------------|
| Validator | This is a server daemon that connects to peer to peer network.  It downloads the Solana blockchain and uploads transactions to a lead validator. [See article for more information](https://solana.com/validators).  Leaders and voting validators require staked SOL. |
| QoS | quality of service; writes to the leader are restricted by stake. |
| QUIC | Validators create connections with each other over UDP using a Google developed protocol called QUIC |
| Bandwidth | a % of TPS (transactions per second) that a validator is capable of sending to the leader. |
| Period | The validator selling bandwidth splits up an epoch into time chunks.  Periods are measured in slots, not unix epoch time. |
| Period Crank | a Crank instruction that pushed the state of the validator bid into the next Period.  Fees are taken and bandwidth allocations are set. |
| Bid | A developer wants to buy bandwidth for an upcoming period.  Before the period starts, the developer deposits funds.  Once a period starts, the bandwidth allocated to the developer by the validator is done in proportion to the amount deposited. |
| Bid Bucket | the funds stored pending a  Period crank.  A fixed % of the bid bucket is taken as the bandwidth fee. |
| Decay Rate | The proportion of the bid bucket taken as a bandwidth fee upon Period crank |



# Actors

## Staked Validators (Seller)

Overall, validators must:
1. set up a bidding space
1. set period variables which include: start time, length, decay rate
1. run a Grpc proxy server that forwards transactions from developers (buyers) to the lead validator.  The proxy implements rate limiting according to the bandwidth allocation stipulated in the bidding account.


Staked validators need to execute the following Solpipe instructions:
1. AddValidator - create a bidding account that lets developers submit bids to the target staked validator
1. AppendPeriod - continously append bidding periods, which includes adjusting the bandwidth fee (% of bid bucket) 

## Developer (Buyer)

Overall, developers must:
1. [continually adjust the balance on his/her bid bucket balance](bidder/bidder.go)
1. [connect to the valdiator proxy via Grpc client](client/client.go)

Developers execute the following Solpipe instructions:
1. InsertBid - deposit/withdraw funds


## Cranker

Anyone can crank so long as the crank fee is set above 0.

[See the cranker code](cranker/cranker.go).



# Golang CLI Test

## Build Binary

Change directory to `./go`.

```bash
make all
```

## Miscellaneous

### Airdrop

```bash
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d EA4d44kdDaCWNcuBjepk8fyRp4WnccGgd1TrseDzcmtY -a 4.5598 -v
```

### Mint

```bash
solana-keygen new -o ./tmp/mint.json
solana-keygen new -o ./tmp/authority.json
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/mint.json) -a 0.1 -v
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/authority.json) -a 0.1 -v
solpipe mint --rpc=http://localhost:8899 --ws=ws://localhost:8900 --payer=./tmp/mint.json --authority=./tmp/mint.json -d 2 -o ./tmp/mint-usdc.json -v
< ./tmp/mint-usdc.json  jq '.id' | sed 's/\"//g'
```
* `-o` is optional in the last command

### Issue Tokens

```bash
solana-keygen new -o ./tmp/bidder.json
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/bidder.json) -a 0.1 -v
solpipe issue --rpc=http://localhost:8899 --ws=ws://localhost:8900 --payer=./tmp/bidder.json --mint=./tmp/mint-usdc.json --owner=$(solana-keygen pubkey ./tmp/bidder.json) -a 1000 -v 
```

Check the new balance with:

```bash
solpipe balance --rpc=http://localhost:8899 --ws=ws://localhost:8900  --mint=$(< ./tmp/mint-usdc.json  jq '.id' | sed 's/\"//g') --owner=$(solana-keygen pubkey ./tmp/bidder.json)  -v
```

## Create accounts

```bash
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/controller-admin.json) -a 100 -v
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/mint.json) -a 100 -v
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/validator.json) -a 100 -v
solpipe airdrop --rpc=http://localhost:8899 --ws=ws://localhost:8900 -d $(solana-keygen pubkey ./tmp/validator-admin.json) -a 100 -v
```


### Create Controller

The help output:

```
Usage: cba-client controller

Create a controller and change the controller settings

Flags:
  -h, --help            Show context-sensitive help.
      --verbose         Set logging to verbose.

      --payer=STRING    the account paying SOL fees
      --admin=STRING    the account with administrative privileges
      --mint=STRING     the mint of the token account to which fees are paid to and by validators
      --rpc=STRING      Connection information to a Solana validator Rpc endpoint with format
                        protocol://host:port (ie http://locahost:8899)
      --ws=STRING       Connection information to a Solana validator Websocket endpoint with format
                        protocol://host:port (ie ws://localhost:8900)
```

```bash
cba-client controller --rpc=http://localhost:8899 --ws=ws://localhost:8900--payer=./tmp/controller-admin.json --admin=./tmp/controller-admin.json --mint=$(< ./tmp/mint-usdc.json  jq '.id' | sed 's/\"//g') -v
```



# Files

* Agents written in Go - see the Github repository
* BPF files (Solana Program) - [See the release page](https://github.com/solpipe/solpipe-tool/releases).


# Development Setups

## Single Node

[See this page](/docs/Development/SingleNode.md)

## Multiple Nodes

[See this page](/docs/Development/MultiNode.md)