# Cranker


Example:

```bash
cba-client --verbose \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900   \
   cranker 30000000 ./localconfig/single/cranker.json 
```


Man page:

```
Usage: cba-client cranker <minbal> <key>

Crank the CBA program

Arguments:
  <minbal>    what is the balance threshold at which the program needs to exit with an error code
  <key>       the file path of the private key

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