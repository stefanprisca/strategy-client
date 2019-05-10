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

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"

	"github.com/golang/protobuf/proto"

	//mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/stretchr/testify/require"

	// "github.com/hyperledger/fabric-sdk-go/test/integration"
	tttPb "github.com/stefanprisca/strategy-protobufs/tictactoe"
)

/*
	Test that will
	1) Create the sdk
	2) Connect both players
	3) Play a game
*/

type runtime struct {
	timestamp int64
	value     float64
}

type scriptStep struct {
	message proto.Message
	player  *TFCClient
}

func scriptTTT1(p1, p2 *TFCClient) []scriptStep {
	return []scriptStep{
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 0, Mark: tttPb.Mark_X}}, player: p1},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 1, Mark: tttPb.Mark_O}}, player: p2},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 4, Mark: tttPb.Mark_X}}, player: p1},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 8, Mark: tttPb.Mark_O}}, player: p2},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 3, Mark: tttPb.Mark_X}}, player: p1},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 5, Mark: tttPb.Mark_O}}, player: p2},
		{message: &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: &tttPb.MoveTrxPayload{Position: 6, Mark: tttPb.Mark_X}}, player: p1},
	}
}

func TestE2E(t *testing.T) {
	runName := "te2e123"
	res, err := execTTTGame(runName, []string{Player1, Player2})
	require.NoError(t, err)
	log.Println(res)
	err = plotRuntimes(res, runName)
	require.NoError(t, err)
}

func TestGoroutinesStatic(t *testing.T) {
	testWithRoutines(t, 4, "testfafa13")
}

func TestGoroutinesIncremental(t *testing.T) {
	testName := "testgrinc"

	for nOfRoutines := 1; nOfRoutines < 10; nOfRoutines *= 2 {
		runName := testName + strconv.Itoa(nOfRoutines)
		testWithRoutines(t, nOfRoutines, runName)
	}
}

func testWithRoutines(t *testing.T, nOfRoutines int, runName string) {

	respChan := make(chan ([]runtime))

	playerPairs := [][]string{
		{Player1, Player2},
		{Player3, Player5},
		{Player4, Player1},
		{Player2, Player3},
		{Player1, Player4},
	}

	log.Printf(" ############# \n\t Starting goRoutine run *%s* with %v routines, and player set %v. \n ##############",
		runName, nOfRoutines, playerPairs)

	batchSize := 1
	batchInterval, err := time.ParseDuration("10s")
	require.NoError(t, err)

	for i := 0; i < nOfRoutines; i += batchSize {
		for j := i; j < i+batchSize; j++ {
			ppI := j % len(playerPairs)
			gameName := runName + strconv.Itoa(j+1)
			go execTTTGameAsync(gameName, respChan, playerPairs[ppI])
		}
		time.Sleep(batchInterval)
	}

	perfResults := [][]runtime{}
	for i := 0; i < nOfRoutines; i++ {
		rts := <-respChan
		perfResults = append(perfResults, rts)
	}

	flatRts := flattenRts(perfResults)
	log.Printf(" ############# \n\t Finished goRoutine run *%s* with runtime results %v. \n ##############",
		runName, flatRts)

	plotRuntimes(flatRts, runName)
}

func execTTTGameAsync(gameName string, respChan chan ([]runtime), chanOrgs []string) {
	res, err := execTTTGame(gameName, chanOrgs)
	respChan <- res

	if err != nil {
		panic(err)
	}
}

func execTTTGame(gameName string, chanOrgs []string) ([]runtime, error) {

	perfResult := []runtime{}

	cfgPath, err := generateChannelArtifacts(gameName, chanOrgs)
	if err != nil {
		return perfResult, err
	}

	players, err := generatePlayers(cfgPath, chanOrgs, gameName)
	if err != nil {
		return perfResult, err
	}
	defer closePlayers(players)

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
	err = startGame(players, cfgPath, ccPath, gameName)
	if err != nil {
		return perfResult, err
	}

	tttScript1 := scriptTTT1(players[0], players[1])
	_, perfResult, err = runScript(tttScript1, gameName)
	if err != nil {
		return perfResult, err
	}

	log.Print("Finished running TTT game.")
	return perfResult, nil
}

func runScript(script []scriptStep, chanName string) ([]channel.Response, []runtime, error) {
	responses := make([]channel.Response, len(script))
	rts := make([]runtime, len(script))
	for i := range script {
		msg := script[i].message
		player := script[i].player
		log.Printf("Executing script step %v", msg)
		trxArgs, err := proto.Marshal(msg)
		if err != nil {
			return responses, rts, err
		}

		st := time.Now()
		r, err := invokeGameChaincode(player, chanName, trxArgs)
		rt := time.Since(st).Seconds()

		rts[i] = runtime{timestamp: st.Unix(), value: rt}
		responses[i] = r
		if err != nil {
			return responses, rts, err
		}
	}

	return responses, rts, nil
}

func flattenRts(runtimes [][]runtime) []runtime {
	result := []runtime{}
	for _, rts := range runtimes {
		result = append(result, rts...)
	}
	return result
}

func plotRuntimes(rts []runtime, name string) error {
	p, err := plot.New()
	if err != nil {
		return err

	}
	xys := make(plotter.XYs, len(rts))

	for i, r := range rts {
		xys[i].X = float64(r.timestamp)
		xys[i].Y = r.value
	}

	err = plotutil.AddScatters(p, xys)
	if err != nil {
		return err

	}

	return p.Save(4*vg.Inch, 4*vg.Inch, "plots/"+name+".png")
}
