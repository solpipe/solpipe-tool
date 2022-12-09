#!/bin/bash

set -eu -o pipefail

GO_BIN="/usr/local/bin/go"
LOCAL_BIN_DIR=$(pwd)/bin
export PATH="$PATH:$LOCAL_BIN_DIR"

# use this to test with ./localconfig/single/* Single Node

decho(){
    1>&2 echo $@
}

copy_keys(){
    DIR=$1
    ( cd ./localconfig/single && ./load.sh $DIR)
    solana -u localhost program deploy --keypair ./localconfig/single/faucet.json ./target/deploy/solmate_cba.so --program-id ./target/deploy/solmate_cba-keypair.json
    sleep 20
}

setup_controller(){
    $GO_BIN test -timeout 120s -run ^TestController$ github.com/solpipe/solpipe-tool/agent/pipeline || true
    sleep 10
}

setup_pipeline(){
    decho "setting up pipeline"
    solana -u localhost airdrop 24 $(solana-keygen pubkey ./localconfig/single/pipeline-admin.json)
    sleep 20
    solpipe --verbose \
        --rpc=http://localhost:8899 \
        --ws=ws://localhost:8900 \
        pipeline create \
        --payer=./localconfig/single/faucet.json \
        ./localconfig/single/pipeline.json \
        ./localconfig/single/pipeline-admin.json \
        1/240 1/10 100 20
}

setup_validator(){
    decho "setting up validator"
    solana -u localhost airdrop 24 $(solana-keygen pubkey ./localconfig/single/validator-admin.json)
    sleep 20
    solana \
        -k ./localconfig/single/faucet.json \
        -u localhost \
        create-stake-account \
        ./localconfig/single/stake.json 200000
    sleep 20
    solana \
        -k ./localconfig/single/faucet.json \
        -u localhost \
        delegate-stake \
        ./localconfig/single/stake.json \
        $(solana-keygen pubkey ./localconfig/single/vote.json)
    sleep 20
    solpipe --verbose  \
        --rpc=http://localhost:8899 \
        --ws=ws://localhost:8900  \
        validator create \
        --payer=./localconfig/single/faucet.json \
        ./localconfig/single/vote.json \
        ./localconfig/single/validator-admin.json  \
        $(solana-keygen pubkey ./localconfig/single/stake.json )

}



CMD=$1
case $CMD in
    reset)
        TEST_LEDGER_DIR=$2
        copy_keys $TEST_LEDGER_DIR
        setup_controller
        setup_pipeline
        setup_validator
    ;;
    *)
        decho "no such CMD=$CMD"
        exit 1
    ;;
esac
