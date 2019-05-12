// Copyright 2019 Stefan Prisca

//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at

//        http://www.apache.org/licenses/LICENSE-2.0

//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
package tfc

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

/*
	Test that will
	1) Create the sdk
	2) Connect both players
	3) Play a game
*/

func TestE2E(t *testing.T) {
	runName := "foofa66"

	respChan := make(chan (bool))

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"

	go execTTTGameAsync(runName, ccPath, respChan, []string{Player1, Player2})
	srv := startProme()
	defer srv.Shutdown(nil)

	<-respChan
}

func TestGoroutinesStatic(t *testing.T) {
	srv := startProme()
	defer srv.Shutdown(nil)
	testWithRoutines(t, 4, "testfafa4132", execDRMAsync)
}

func TestGoroutinesIncremental(t *testing.T) {
	testName := "testgrinc66"
	srv := startProme()
	defer srv.Shutdown(nil)

	for nOfRoutines := 1; nOfRoutines < 10; nOfRoutines *= 2 {
		runName := testName + strconv.Itoa(nOfRoutines)
		testWithRoutines(t, nOfRoutines, runName, execTTTGameAsync)
	}
}

func testWithRoutines(t *testing.T, nOfRoutines int, runName string, asyncExec asyncExecutor) {

	respChan := make(chan (bool))

	playerPairs := [][]string{
		{Player1, Player2, Player5},
		{Player3, Player5, Player2},
		{Player4, Player1, Player3},
		{Player2, Player3, Player1},
		{Player1, Player4, Player2},
	}

	log.Printf(" ############# \n\t Starting goRoutine run *%s* with %v routines, and player set %v. \n ##############",
		runName, nOfRoutines, playerPairs)

	batchSize := 1
	batchInterval, err := time.ParseDuration("10s")
	require.NoError(t, err)
	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"

	for i := 0; i < nOfRoutines; i += batchSize {
		for j := i; j < i+batchSize; j++ {
			ppI := j % len(playerPairs)
			gameName := runName + strconv.Itoa(j+1)
			go asyncExec(gameName, ccPath, respChan, playerPairs[ppI])
		}
		time.Sleep(batchInterval)
	}

	log.Printf(" ############# \n\t Finished goRoutine run *%s* . \n ##############",
		runName)
}
