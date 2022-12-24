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



## Capacity Market

| *Solpipe Definition* |
| --- |
| ![Flow Management](/docs/files/flow.png "Flow management") |



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