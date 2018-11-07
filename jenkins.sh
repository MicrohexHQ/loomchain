#!/bin/bash

set -ex

PKG=github.com/loomnetwork/loomchain

# setup temp GOPATH
export GOPATH=/tmp/gopath-$BUILD_TAG
export
export PATH=$GOPATH:$PATH:/var/lib/jenkins/workspace/commongopath/bin

LOOM_SRC=$GOPATH/src/$PKG
mkdir -p $LOOM_SRC
rsync -r --delete . $LOOM_SRC

cd $LOOM_SRC
make clean
make deps
make
make validators-tool
#make test
#make test-no-evm
go test -timeout 20m -v -tags "evm" github.com/loomnetwork/loomchain/e2e -run ^TestContractCoin$
make tgoracle
