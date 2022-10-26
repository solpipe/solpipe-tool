# Pipeline


# Run Agent


## Example Usage

```bash
cba-client --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   pipeline agent \
   --crank_rate=1/100 \
   --decay_rate=1/100 \
   --payout_share=95/100 \
   --admin_url="unix:///tmp/pipeline.socket" \
  2Tb48kmdnsnRKcuHDb5iVjeKvZbLrS7U3rxnMf9t2rC7 \
  ./localconfig/single/pipeline-admin.json
```


## Man page

```
Usage: cba-client pipeline agent <id> <admin>

run a Pipeline Agent

Arguments:
  <id>       the Pipeline ID
  <admin>    the Pipeline admin

Flags:
  -h, --help                         Show context-sensitive help.
  -v, --verbose                      Set logging to verbose.
      --program-id="2nV2HN9eaaoyk4WmiiEtUShup9hVQ21rNawfor9qoqam"
                                     Program ID for the CBA Solana program
      --version=VERSION              What version is the controller
      --rpc=RPC-URL                  Connection information to a Solana validator Rpc endpoint with format protocol://host:port (ie
                                     http://localhost:8899)
      --ws=WS-URL                    Connection information to a Solana validator Websocket endpoint with format protocol://host:port
                                     (ie ws://localhost:8900)
      --apikey=API-KEY               An API Key used to connect to an RPC Provider

      --crank_rate=STRING            the crank rate in the form NUMERATOR/DENOMINATOR
      --decay_rate=STRING            the decay rate in the form NUMERATOR/DENOMINATOR
      --payout_share=STRING          the payout share in the form NUMERATORDENOMINATOR
  -u, --admin_url=STRING             port on which to listen for Grpc connections from administrators. Use tcp://host:port or
                                     unix:///var/run/admin.socket
  -b, --balance=UINT-64              set the minimum balance threshold
      --program_id_cba=PUBLIC-KEY    Specify the program id for the CBA program
```


# Status

## Example

```bash
cba-client   --rpc=http://localhost:8899    --ws=ws://localhost:8900    --verbose  pipeline status 2Tb48kmdnsnRKcuHDb5iVjeKvZbLrS7U3rxnMf9t2rC7
```

## Man Page


```
Usage: cba-client pipeline status <id>

Print the admin, token balance of the controller

Arguments:
  <id>    the Pipeline ID

Flags:
  -h, --help                         Show context-sensitive help.
  -v, --verbose                      Set logging to verbose.
      --program-id="2nV2HN9eaaoyk4WmiiEtUShup9hVQ21rNawfor9qoqam"
                                     Program ID for the CBA Solana program
      --version=VERSION              What version is the controller
      --rpc=RPC-URL                  Connection information to a Solana validator Rpc endpoint with format protocol://host:port (ie
                                     http://localhost:8899)
      --ws=WS-URL                    Connection information to a Solana validator Websocket endpoint with format protocol://host:port
                                     (ie ws://localhost:8900)
      --apikey=API-KEY               An API Key used to connect to an RPC Provider

      --program_id_cba=PUBLIC-KEY    Specify the program id for the CBA program
```