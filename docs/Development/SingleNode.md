# Single Node Testing

Test Solpipe using a single validator and a single staker.


# Setup

## Terminal 1 - Test Validator

Run the test validator.

```bash
mkdir $HOME/tmp
cd $HOME/tmp
solana-test-validator
```

Press `ctrl+C` to stop the validator.  Do `rm -r $HOME/tmp/test-ledger` and restart `solana-test-validator` to reset all of the data.

## Terminal 2 - Deploy Program

Deploy the program locally.  We assume the `solpipe-tool.tar.gz` files is in `$HOME/Downloads`.

```bash
cd $HOME/work
git clone ssh://git@gitlab.com/eflam/solpipe-tool
cd solpipe-tool
git checkout master
mkdir -p ./target/deploy
tar -xvzf $HOME/Downloads/solpipe-tool.tar.gz  -C ./
( ./localconfig/single/load.sh $HOME/tmp/test-ledger )
solana -u localhost program deploy --keypair ./localconfig/single/faucet.json ./target/deploy/solmate_cba.so --program-id ./target/deploy/solmate_cba-keypair.json
```

## Setup via Test

Run this in VS Code.

Run the test in `./agent/pipeline/controller_test.go`   This will create the Controller on chain.


## Terminal 3 - Pipeline

### Create a Pipeline

Create a pipeline:

```bash
solpipe --verbose \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900 \
   pipeline create \
   --payer=./localconfig/single/faucet.json \
   ./localconfig/single/pipeline.json \
   ./localconfig/single/pipeline-admin.json \
   1/240 1/10 1/2 100
```

### Pipeline Agent - Daemon

Run the pipeline:

```bash
solpipe --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   pipeline agent \
   --crank_rate=1/100 \
   --decay_rate=1/100 \
   --payout_share=95/100 \
   --clear_listen=127.0.0.1:50051 \
   --admin_url="unix:///tmp/pipeline.socket" \
  $(solana-keygen pubkey ./localconfig/single/pipeline.json) \
  ./localconfig/single/pipeline-admin.json
```
* make sure `/tmp/pipeline.socket` does not exist prior to running this command

### Pipeline Update

```bash
solpipe --verbose \
  --rpc=http://localhost:8899 \
  --ws=ws://localhost:8900 \
  pipeline update \
  --payer=./localconfig/single/faucet.json \
  $(solana-keygen pubkey ./localconfig/single/pipeline.json ) \
  ./localconfig/single/pipeline-admin.json \
  1/240 1/10 1/3 250
```

## Terminal 4 - Validator

### Delegate Stake

We need to have a stake account to prove that the vote account is legitamite.  Anchor does not have the ability to parse voting accounts.  Therefore, we need to add a stake account to the AddValidator instruction.

Create a stake account:

```bash
solana \
   -k ./localconfig/single/faucet.json \
   -u localhost \
   create-stake-account \
   ./localconfig/single/stake.json 200000
```

Delegate stake to the validator:

```bash
solana \
   -k ./localconfig/single/faucet.json \
   -u localhost \
   delegate-stake \
   ./localconfig/single/stake.json \
   $(solana-keygen pubkey ./localconfig/single/vote.json)
```

### Create Validator

Create the validator on-chain.  Make sure validator admin has sufficient lamports to pay rent for the Validator Member account.

```bash
solpipe --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   validator create \
   --payer=./localconfig/single/faucet.json \
   ./localconfig/single/vote.json \
   ./localconfig/single/validator-admin.json  \
   $(solana-keygen pubkey ./localconfig/single/stake.json )
```


Run the validator agent.

```bash
solpipe --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   validator agent --help \
   --clear_listen=127.0.0.1:50051 \
   --admin_url="unix:///tmp/pipeline.socket" \
   $(solana-keygen pubkey ./localconfig/single/pipeline.json) \
  ./localconfig/single/pipeline-admin.json
```


## Terminal 5 - Website

```bash
solpipe --verbose  \
   --rpc=http://localhost:8899 \
   --ws=ws://localhost:8900  \
   web \
   --frontend=http://localhost:3001 \
   4001
```