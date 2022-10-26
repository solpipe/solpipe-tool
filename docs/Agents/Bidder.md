# Bidder


# Agent

## Example


```bash
./go/bin/cba-client   --verbose \
   --rpc=http://localhost:8899  \
   --ws=ws://localhost:8900   \
   bid agent \
   ./localconfig/single/bidder-1.json ./localconfig/single/bidder-1-config.json
```


## Man Page

```
Usage: cba-client bid agent <key> <config>

run a Bidding Agent

Arguments:
  <key>       the private key of the wallet that owns tokens used to bid on bandwidth and also authenticates over grpc with the staked
              validator
  <config>    the file path to the configuration file

Flags:
  -h, --help               Show context-sensitive help.
  -v, --verbose            Set logging to verbose.
      --program-id="2nV2HN9eaaoyk4WmiiEtUShup9hVQ21rNawfor9qoqam"
                           Program ID for the CBA Solana program
      --version=VERSION    What version is the controller
      --rpc=RPC-URL        Connection information to a Solana validator Rpc endpoint with format protocol://host:port (ie
                           http://localhost:8899)
      --ws=WS-URL          Connection information to a Solana validator Websocket endpoint with format protocol://host:port (ie
                           ws://localhost:8900)
      --apikey=API-KEY     An API Key used to connect to an RPC Provider
```

# Setup

???