# Validator Agnet

Events to care about:

### pipeline.OnPayout

1. If no receipt, execute *create receipt* .
1. Let pipeline agent connect to us over TOR.


```bash
solpipe --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   validator agent --help \
   --clear_listen=127.0.0.1:50051 \
   --admin_url="unix:///tmp/pipeline.socket" \
  2Tb48kmdnsnRKcuHDb5iVjeKvZbLrS7U3rxnMf9t2rC7 \
  ./localconfig/single/pipeline-admin.json
```