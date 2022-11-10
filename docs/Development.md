# Developing for Solpipe

We have to spin up a test validator and run Go tests against said validators.


# Files

* Agents written in Go - see the Github repository
* BPF files (Solana Program) - [See the release page](https://github.com/solpipe/solpipe-tool/releases).

# Setup


## Terminal 1

Run the test validator.

```bash
mkdir $HOME/tmp
cd $HOME/tmp
solana-test-validator
```

Press `ctrl+C` to stop the validator.  Do `rm -r $HOME/tmp/test-ledger` and restart `solana-test-validator` to reset all of the data.

## Terminal 2

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

## Terminal 3

Run this in VS Code.

1. Run the test in `./agent/pipeline/controller_test.go`   This will create the Controller on chain.
1. Run the test in `./agent/pipeline/pipeline_test.go`.  This will create a pipeline.