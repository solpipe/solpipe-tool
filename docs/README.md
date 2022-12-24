# Solpipe

Solpipe is a framework on Solana that lets users create a market for capacity-constrained service.  Solpipe is flexible, convenient, and privacy protecting.  Solpipe can be used in many different business contexts.  Examples of such markets are:

* [Validators on Solana auctioning Transaction bandwidth](https://docs.solana.com/terminology#tps).  Said [bandwidth is contrained by QoS and QUIC](https://github.com/solana-labs/solana/issues/23211).
* Generalized API gateways where API calls by users are rate limited
* Electricity markets where users sell electricity generation capacity

The Solpipe is composed of multiple code bases:

* a Solana Program (i.e. smart contract written in Rust)
* this Go repository for managing onchain state
* a React library for viewing the status of the market

**THIS CODE IS STILL IN ALPHA STATE**

The main objective of the framework is:

* to let buyers and sellers place/take bids and settle cash on chain
* to record/meter usage on chain using mutually signed receipt accounts
* if the capacity is an internet deliverable such as an API gateway or JSON RPC service, then set up a [a TOR](https://en.wikipedia.org/wiki/Tor_(network)) connection and an optional clear net peer to peer connection mutually authenticated over TLS using the respective Solana keypairs used in the bidding process (see [Proxy.md](Proxy.md) for details)

## Capacity Market

The definition of a capacity market is that there is a consumable service or product (oil, API calls, etc) that is used at some rate *consumable/time*  (e.g. MW for electricity, Tx/s for Solana  ) over some period time (e.g. 1 hour between 00:00 UTC and 01:00 UTC).

| *Solpipe Definition* |
| --- |
| ![Flow Management](/docs/files/flow.png "Flow management") |

In said market, single or multiple suppliers define time periods and offer capacity priced in *money/consumable* (e.g. 21 USD/KwH for electricity, 0.004 USD/tx for Solana).

| *Market Supply and Demand* |
| --- |
| ![Market Supply and Demand](/docs/files/market.png "Market Supply and Demand") |


# How Solpipe Works

Solpipe is a capacity market with state stored onchain.  Agents (Go programs in the repository) track and update onchain state.

| *Terms* | *Definition* |
| --- | --- |
| Bidder | The person bidding for capacity [see details here](../Agents/Bidder.md) |
| Pipeline | The person offering capacity | [see details here](../Agents/Pipeline.md). For [Solana transaction processing capacity](https://docs.solana.com/terminology#tps), see also [Validator](../Agents/Validator.md) and [Staker](../Agents/Staker.md) |
| TIME_PERIOD | a time period for which capacity is being bid |
| BID_TABLE | an account belonging to the TIME_PERIOD for which capacity is being bid |
| ALLOCATION | the share of capacity being allocated to a Bidder upon start of the TIME_PERIOD |
| PC_MINT | the denomination of the funds that can be deposited in the BID_TABLE |
| BID_RECEIPT | an onchain account storing usage information that is mutually signed by the Pipeline and Bidder |
| TOR | [stands for the onion router](https://www.torproject.org/) |

There is a Solana Program that implements a bidding system.  Pipelines create bidding periods that span a number of slots starting at either the current slot or some slot in the future.  Off-chain, bidders can calculate the real time capacity of a Pipeline and forecast the capacity of said Pipeline during the time period being bid on.


### Bid Cycle

||
| --- |
| **PICTURE BELOW IS SLIGHTLY OUT OF DATE.  THERE IS NO DECAY RATE. THERE IS ONE BID BUCKET PER PERIOD.** |
![Bid Cycle Diagram](/docs/files/bid-cycle.png "Bid Cycle")

A Pipeline creates a TIME_PERIOD to which a unique BID_TABLE is attached.  Bidders can deposit funds into BID_TABLE up until the start of the TIME_PERIOD.

The Bidder connects to Pipeline and maintains connection for the duration of TIME_PERIOD.  See [Proxy](Proxy.md) for details.  All necessary connection information is derived from the public keys used in the onchain bidding process.

During TIME_PERIOD, the Bidder can make API calls such as send_transaction, but is rate limited by the ALLOCATION.  The usage is recorded onchain in BID_RECEIPT.

# Market Examples

The main purpose of Solpipe is to auction TPS capacity.  However, SaaS businesses providing rate-limited API services can use the same tooling to earn revenue.

Customers have the option of concealing their IP addresses by consuming services over the default TOR connections.

## Solana

| *Solana* |
| --- |
| ![Solana Example](/docs/files/eg-solana.png "Solana Example") |

|||
| --- | --- |
| **Capacity** | transactions per second |
| **Volume** | total transactions submitted over a fixed time period |
| **Price** | USD per transaction during the fixed time period |

## API Billing Gateway

| *Solana* |
| --- |
| ![API Billing Gateway](/docs/files/eg-api.png "API Billing Gateway") |

|||
| --- | --- |
| **Capacity** | transactions per second |
| **Volume** | total transactions submitted over a fixed time period |
| **Price** | USD per transaction during the fixed time period |

## Electricity Generation Market

| *Solana* |
| --- |
| ![Electricity Generation Market](/docs/files/eg-power.png "Electricity Generation Market") |

|||
| --- | --- |
| **Capacity** | transactions per second |
| **Volume** | total transactions submitted over a fixed time period |
| **Price** | USD per transaction during the fixed time period |