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
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

/*
	Test that will
	1) Create the sdk
	2) Connect both players
	3) Play a game
*/

var playerPairs = [][]string{
	{Player1, Player2, Player3},
	{Player3, Player5, Player4},
	{Player4, Player1, Player2},
	{Player2, Player3, Player5},
	{Player5, Player4, Player1},
	{Player2, Player3, Player1},
	{Player3, Player5, Player2},
	{Player5, Player1, Player4},
}

func TestE2E(t *testing.T) {
	runName := "te12328"

	promeShutdown := startProme()
	defer promeShutdown()

	respChan := make(chan (error), 10)
	orgsIn := make(chan ([]string), 10)
	orgsOut := make(chan ([]string), 10)

	orgsIn <- []string{Player1, Player2, Player3}

	execTFCGameAsync(runName, respChan, orgsIn, orgsOut)
	<-orgsOut

	<-respChan
}

func TestGoroutinesStatic(t *testing.T) {
	testName := "testgrinc"
	rand.Seed(time.Now().Unix())
	testName += strconv.Itoa(rand.Int() % 100)

	promeShutdown := startProme()
	defer promeShutdown()

	testWithRoutines(t, 8, testName, execTFCGameAsync, playerPairs)
}

func TestGoroutinesIncremental(t *testing.T) {
	testName := "ti"
	rand.Seed(time.Now().Unix())
	testName += strconv.Itoa(rand.Int() % 100)
	promeShutdown := startProme()
	defer promeShutdown()

	testWithRoutines(t, 1, "tfc"+testName, execTFCGameAsync, playerPairs)

	for nOfRoutines := 2; nOfRoutines <= 32; nOfRoutines *= 2 {

		nOfTFC := nOfRoutines/2 - 1
		tfcDone := make(chan (bool), nOfTFC+1)
		nOfTTT := nOfRoutines/2 + 1
		tttDone := make(chan (bool), nOfTTT+1)

		runName := fmt.Sprintf("%s%d", testName, nOfRoutines)
		go testWithRoutinesAsync(t, nOfTFC, "tfc"+runName, execTFCGameAsync, playerPairs[:4], tfcDone)
		go testWithRoutinesAsync(t, nOfTTT, "ttt"+runName, execTTTGameAsync, playerPairs[4:], tttDone)

		log.Println("Waiting for TTT to be done....")
		<-tttDone

		log.Println("Waiting for TFC to be done....")
		<-tfcDone
	}
}

func testWithRoutinesAsync(t *testing.T, nOfRoutines int, runName string, asyncExec asyncExecutor, playerPairs [][]string, done chan (bool)) {
	testWithRoutines(t, nOfRoutines, runName, asyncExec, playerPairs)
	done <- true
}

func testWithRoutines(t *testing.T, nOfRoutines int, runName string, asyncExec asyncExecutor, playerPairs [][]string) {

	if nOfRoutines == 0 {
		return
	}

	errOutChan := make(chan (error))
	defer close(errOutChan)
	orgsIn := make(chan ([]string), len(playerPairs))
	defer close(orgsIn)
	orgsOut := make(chan ([]string), len(playerPairs)+1)
	defer close(orgsOut)

	for _, pp := range playerPairs {
		orgsIn <- pp
	}

	log.Printf(" ############# \n\t Starting goRoutine run *%s* with %v routines, and player set %v. \n ##############",
		runName, nOfRoutines, playerPairs)

	for i := 0; i < nOfRoutines; i++ {
		gameName := runName + strconv.Itoa(i+1)
		go asyncExec(gameName, errOutChan, orgsIn, orgsOut)
		orgsIn <- (<-orgsOut)
	}

	for i := 0; i < nOfRoutines; i++ {
		<-errOutChan
	}

	log.Printf(" ############# \n\t Finished goRoutine run *%s* . \n ##############",
		runName)
}
