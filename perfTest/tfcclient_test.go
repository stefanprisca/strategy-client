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
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/golang/protobuf/proto"

	//mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/stretchr/testify/require"

	// "github.com/hyperledger/fabric-sdk-go/test/integration"
	promClient "github.com/prometheus/client_golang/prometheus"
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

var labelNames = []string{"Foo"}
var promeHist *prometheus.Histogram

func startProme() http.Server {

	srv := http.Server{Addr: ":9009"}
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		httpError := srv.ListenAndServe()
		if httpError != nil {
			log.Println("While serving HTTP: ", httpError)
		}
	}()

	promeHist = prometheus.NewHistogramFrom(
		promClient.HistogramOpts{
			Namespace: "tfc",
			Subsystem: "testing",
			Name:      "runtime",
			Help:      "No help",
		}, labelNames)

	return srv
}

func TestE2E(t *testing.T) {
	runName := "te6674467se4"

	respChan := make(chan ([]runtime))

	go execTTTGameAsync(runName, respChan, []string{Player1, Player2})
	srv := startProme()
	defer srv.Shutdown(nil)

	<-respChan
}

func TestGoroutinesStatic(t *testing.T) {
	testWithRoutines(t, 4, "testfafa41")
}

func TestGoroutinesIncremental(t *testing.T) {
	testName := "testgrinc5"
	srv := startProme()
	defer srv.Shutdown(nil)

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

	log.Printf(" ############# \n\t Finished goRoutine run *%s* . \n ##############",
		runName)
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

	for _, p := range players {
		p.Metrics = promeHist
	}

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

	log.Printf("Finished running TTT game. Wrote metrics out")
	return perfResult, err
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
		_, err = invokeGameChaincode(player, chanName, trxArgs)
		rt := time.Since(st).Seconds()

		player.Metrics.
			With(labelNames...).
			Observe(rt)

		ms := rand.Intn(1000) + 500
		stepInterval, _ := time.ParseDuration(fmt.Sprintf("%vms", ms))
		time.Sleep(stepInterval)
		if err != nil {
			return responses, rts, err
		}
	}

	return responses, rts, nil
}
