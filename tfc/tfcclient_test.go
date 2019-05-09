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
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gonum/stat"

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

type perfResult struct {
	runtimes []float64
	peerSize int
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
	respChan := make(chan (perfResult))
	go execTTTGame(t, "1foo1", respChan, []string{Player1, Player2})
	go execTTTGame(t, "1faa2", respChan, []string{Player2, Player3})
	go execTTTGame(t, "1fop3", respChan, []string{Player5, Player4})
	go execTTTGame(t, "1bop4", respChan, []string{Player3, Player1})

	perfResultMeans := []float64{}
	for x := range respChan {
		rts := x.runtimes
		mean := stat.Mean(rts, nil)
		perfResultMeans = append(perfResultMeans, mean)

		if len(perfResultMeans) == 3 {
			break
		}
	}

	log.Println(perfResultMeans)
}

func execTTTGame(t *testing.T, name string, resultChan chan (perfResult), chanOrgs []string) {

	gameName := name

	cfgPath, err := generateChannelArtifacts(gameName, chanOrgs)
	require.NoError(t, err)

	players, err := generatePlayers(cfgPath, chanOrgs, gameName)
	require.NoError(t, err)
	defer closePlayers(players)

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
	err = startGame(players, cfgPath, ccPath, gameName)
	require.NoError(t, err)

	tttScript1 := scriptTTT1(players[0], players[1])
	_, perfResult, err := runScript(tttScript1, gameName)
	require.NoError(t, err)

	log.Printf("Executed test with average runtime %v", stat.Mean(perfResult.runtimes, nil))
	resultChan <- perfResult
}

func runScript(script []scriptStep, chanName string) ([]channel.Response, perfResult, error) {
	responses := make([]channel.Response, len(script))
	perf := perfResult{runtimes: make([]float64, len(script))}
	for i := range script {
		msg := script[i].message
		player := script[i].player
		log.Printf("Executing script step %v", msg)
		trxArgs, err := proto.Marshal(msg)
		if err != nil {
			return responses, perf, err
		}

		st := time.Now()
		r, err := invokeGameChaincode(player, chanName, trxArgs)
		rt := time.Since(st).Seconds()
		perf.runtimes[i] = rt
		responses[i] = r
		if err != nil {
			return responses, perf, err
		}
	}

	return responses, perf, nil

}
