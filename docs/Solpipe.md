# Solpipe

Solpipe is a framework on Solana that lets users create a market for capacity-constrained service.  Examples of such markets are:

* Validators on Solana auctioning Transaction bandwidth.  Said bandwidth is contrained by QoS and QUIC.
* Generalized API gateways where API calls by users are rate limited
* Electricity markets where users sell electricity generation capacity

The Solpipe is composed of multiple code bases:

* the Solana Program itself
* this Go repository for managing onchain state
* a React library for viewing the status of the market

**THIS CODE IS STILL IN ALPHA STATE**

The main objective of the framework is:

* let buyers and sellers place/take bids and settle cash on chain
* record/meter usage on chain using mutually signed receipt accounts
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

| *Agent* | *Description* |
| --- | --- |
| hi ||


## Life Cycle

![Bid Cycle Diagram](/docs/files/bid-cycle.png "Bid Cycle")


# Examples

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