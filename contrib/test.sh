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
    decho "deploying program"
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

prep_validator(){
    DIR=$1
    PORT=$2
    mkdir -p $DIR/ledger
    echo $PORT >$DIR/port.txt
    solana-keygen new --no-bip39-passphrase -o $DIR/id.json
    solana-keygen new --no-bip39-passphrase -o $DIR/vote.json
    solana-keygen new --no-bip39-passphrase -o $DIR/vote-admin.json
    solana-keygen new --no-bip39-passphrase -o $DIR/validator.json
    solana-keygen new --no-bip39-passphrase -o $DIR/validator-admin.json

    cp -r $DIR/vote.json ./localconfig/single/vote-$PORT.json
    cp -r $DIR/validator.json ./localconfig/single/validator-$PORT.json
    cp -r $DIR/validator-admin.json ./localconfig/single/validator-admin-$PORT.json

    solana -u localhost airdrop 24 $DIR/id.json
    solana -u localhost airdrop 24 $DIR/validator-admin.json
    # account, identity, withdrawer
    solana -u localhost -k $DIR/validator-admin.json create-vote-account $DIR/vote.json $DIR/id.json $DIR/vote-admin.json
}

run_validator(){
    DIR=$1
    ls $DIR/id.json
    ls $DIR/vote.json
    ls $DIR/ledger
    PORT=$(cat $DIR/port.txt)
    
    exec solana-validator \
		-l $DIR/ledger \
		--log $DIR/out.log \
		--private-rpc \
		--full-rpc-api \
		--rpc-port $PORT \
		--entrypoint 127.0.0.1:8001 \
		--tpu-use-quic \
		-i $DIR/id.json \
		--vote-account $DIR/vote.json 
}


CMD=$1
case $CMD in
    init)
        TEST_LEDGER_DIR=$2
        copy_keys $TEST_LEDGER_DIR
        setup_controller
        setup_pipeline
        setup_validator
    ;;
    prepval)
        DIR=$2
        PORT=$3
        prep_validator $DIR $PORT
    ;;
    run)
        DIR=$2
        echo "type the following command:"
        exec echo solana-test-validator --ledger $DIR --gossip-port 8001 
    ;;
    runval)
        DIR=$2
        run_validator $DIR
    ;;
    *)
        decho "no such CMD=$CMD"
        exit 1
    ;;
esac
