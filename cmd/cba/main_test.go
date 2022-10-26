package main_test

import (
	"fmt"
	"os"
	"testing"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/util"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func TestTop(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	log.SetReportCaller(true)
	log.SetLevel(log.DebugLevel)
	err = util.SetProgramID()
	if err != nil {
		t.Fatal(err)
	}
	validatorCount := 1

	log.Infof("program id=%s", cba.ProgramID.String())
	os.Setenv("VALIDATOR_COUNT", fmt.Sprintf("%d", validatorCount-1))
	os.Setenv("PEOPLE_COUNT", "200")
	os.Setenv("STAKER_COUNT", "5")

	t.Run("Controller", TestController)
	for i := 0; i < validatorCount; i++ {
		t.Run("Validator", TestValidator)
	}
	//t.Run("Staker", TestStaker)

	pipelineCount := 1
	for i := 0; i < pipelineCount; i++ {
		t.Run("Pipeline", TestPipeline)
	}
	//t.Run("Cranker", TestCranker)
}
