all: clean build
.PHONY: all cba proxy

cba: ./cmd/cba/main.go
	go build -gcflags=$(SKAFFOLD_GO_GCFLAGS) -mod=readonly  -o ../bin/cba-client ./cmd/cba

#proxy: ./staker/cmd/proxy/main.go
#	cd staker && go build -gcflags=$(SKAFFOLD_GO_GCFLAGS) -mod=readonly  -o ../bin/cba-proxy ./cmd/proxy

#tools: ./staker/cmd/tools/main.go
#	cd staker && go build -gcflags=$(SKAFFOLD_GO_GCFLAGS) -mod=readonly  -o ../bin/cba-tools ./cmd/tools

prep:
	mkdir bin 

#build: prep cba proxy tools
build: prep cba

clean:
	rm -rf bin
