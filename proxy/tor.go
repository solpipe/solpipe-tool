package proxy

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	_ "embed"

	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
)

//go:embed files/torrc
var torrC []byte

//go:embed files/torrc.client
var torrCForClient []byte

func SetupTor(ctx context.Context, isClient bool) (torMgr *tor.Tor, err error) {

	var f *os.File
	f, err = ioutil.TempFile(os.TempDir(), "torrc-*")
	if err != nil {
		return
	}
	torrcFp := f.Name()
	err = f.Chmod(0600)
	if err != nil {
		return
	}
	var torConfigContent []byte
	if isClient {
		torConfigContent = torrCForClient
	} else {
		torConfigContent = torrC
	}
	_, err = io.Copy(f, bytes.NewBuffer(torConfigContent))
	if err != nil {
		return
	}
	err = f.Close()
	if err != nil {
		return
	}
	go loopDeleteFile(ctx, torrcFp)

	dataDir, err := ioutil.TempDir(os.TempDir(), "tor-*")
	if err != nil {
		return
	}

	_, err = ioutil.TempFile(os.TempDir(), "tor-*.log")
	if err != nil {
		return
	}

	log.Debug("tor config:")
	os.Stderr.WriteString(string(torConfigContent))
	if isClient {
		log.Debug("connecting as tor client")
		torMgr, err = tor.Start(ctx, &tor.StartConf{
			EnableNetwork:     true,
			DebugWriter:       os.Stderr,
			TempDataDirBase:   dataDir,
			RetainTempDataDir: false,
			//TorrcFile:         torrcFp,
			NoAutoSocksPort: false,
		})
	} else {
		log.Debug("connecting as tor server")
		torMgr, err = tor.Start(ctx, &tor.StartConf{
			EnableNetwork:     true,
			DebugWriter:       os.Stderr,
			TempDataDirBase:   dataDir,
			RetainTempDataDir: false,
			//TorrcFile:         torrcFp,
			NoAutoSocksPort: false,
		})
	}

	if err != nil {
		return
	}
	//go loopKillTor(ctx, torMgr)
	return
}

func loopDeleteFile(ctx context.Context, fp string) {
	<-ctx.Done()
	os.Remove(fp)
}
