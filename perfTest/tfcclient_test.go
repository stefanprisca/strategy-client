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

func TestE2E(t *testing.T) {
	runName := "te1232"

	promeShutdown := startProme()
	defer promeShutdown()

	respChan := make(chan (bool), 10)
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
	testName += strconv.Itoa(rand.Int())

	promeShutdown := startProme()
	defer promeShutdown()

	testWithRoutines(t, 8, testName, execTTTGameAsync)
}

func TestGoroutinesIncremental(t *testing.T) {
	testName := "testgri2nc"
	rand.Seed(time.Now().Unix())
	testName += strconv.Itoa(rand.Int())
	promeShutdown := startProme()
	defer promeShutdown()

	nOfTfc := 3
	tfcDone := make(chan (bool), nOfTfc)
	go testWithRoutinesAsync(t, nOfTfc, "tfc"+testName, execTFCGameAsync, tfcDone)

	for nOfRoutines := 2; nOfRoutines < 32; nOfRoutines *= 2 {
		runName := testName + strconv.Itoa(nOfRoutines)
		testWithRoutines(t, nOfRoutines, "ttt"+runName, execTTTGameAsync)
	}

	<-tfcDone
}

func testWithRoutinesAsync(t *testing.T, nOfRoutines int, runName string, asyncExec asyncExecutor, done chan (bool)) {
	testWithRoutines(t, nOfRoutines, runName, asyncExec)
	done <- true
}

func testWithRoutines(t *testing.T, nOfRoutines int, runName string, asyncExec asyncExecutor) {

	playerPairs := [][]string{
		{Player1, Player2, Player3},
		{Player3, Player5, Player4},
		{Player4, Player1, Player2},
		{Player2, Player3, Player5},
		{Player5, Player4, Player1},
	}
	respChan := make(chan (bool))
	defer close(respChan)
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
		go asyncExec(gameName, respChan, orgsIn, orgsOut)
		orgsIn <- (<-orgsOut)
	}

	for i := 0; i < nOfRoutines; i++ {
		<-respChan
	}

	log.Printf(" ############# \n\t Finished goRoutine run *%s* . \n ##############",
		runName)
}
