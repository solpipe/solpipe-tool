package util

import (
	"encoding/json"
	"io"
	"os/exec"
)

func ParseStdout(cmd *exec.Cmd, output interface{}) error {
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmdC := make(chan error, 1)
	jsonC := make(chan error, 1)
	go loopParseStdoutCmd(cmd, cmdC)
	go loopParseStdoutJson(json.NewDecoder(reader), output, jsonC)
	var err error
	select {
	case err = <-cmdC:
		if err == nil {
			err = <-jsonC
		}
	case err = <-jsonC:
		if err == nil {
			err = <-cmdC
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func loopParseStdoutJson(decoder *json.Decoder, output interface{}, errorC chan<- error) {
	errorC <- decoder.Decode(output)
}

func loopParseStdoutCmd(cmd *exec.Cmd, errorC chan<- error) {
	errorC <- cmd.Run()
}
