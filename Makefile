all: clean build
.PHONY: all cba proxy

cba: ./cmd/cba/main.go
	go build -gcflags=$(SKAFFOLD_GO_GCFLAGS) -mod=readonly  -o ./bin/solpipe ./cmd/cba

prep:
	mkdir bin 

#build: prep cba proxy tools
build: prep cba

clean:
	rm -rf bin
