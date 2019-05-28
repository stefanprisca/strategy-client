package tfc

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/stefanprisca/strategy-code/tfc"
	tfcCC "github.com/stefanprisca/strategy-code/tfc"
	tfcPb "github.com/stefanprisca/strategy-protobufs/tfc"
	tttPb "github.com/stefanprisca/strategy-protobufs/tictactoe"
)

type asyncExecutor = func(gameName string, respChan chan (error), orgsIn chan ([]string), orgsOut chan ([]string))

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

type asyncAcriptAllianceGenerator func(int, string, chan error)

type ally struct {
	*TFCClient
	Color tfcPb.Player
}

func scriptTFC1(p1, p2, p3 *TFCClient) ([]scriptStep, asyncAcriptAllianceGenerator) {

	p1C, p2C, p3C := tfcPb.Player_RED, tfcPb.Player_GREEN, tfcPb.Player_BLUE

	colors := map[string]tfcPb.Player{
		p1.OrgID: p1C, p2.OrgID: p2C, p3.OrgID: p3C,
	}

	s := []scriptStep{
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p1C).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p2C).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p3C).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p1C, p2C, tfcPb.Resource_HILL, 2).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p1C, p3C, tfcPb.Resource_HILL, 2).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p2C, p1C, tfcPb.Resource_HILL, 2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p2C, p3C, tfcPb.Resource_HILL, 2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p2C, p3C, tfcPb.Resource_FOREST, -2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p1C, tfcPb.Resource_HILL, 2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p2C, tfcPb.Resource_HILL, 2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p2C, tfcPb.Resource_FOREST, -2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
	}

	for i := 0; i < 2; i++ {
		s = append(s, s[3:]...)
	}

	return s, func(i int, gameName string, eOut chan error) {
		a1 := s[i%3].player
		a2 := s[(i+1)%3].player

		allies := []*ally{
			{a1, colors[a1.OrgID]},
			{a2, colors[a2.OrgID]},
		}
		allianceUUID := uint32(100 + i)

		terms := []*tfcPb.GameContractTrxArgs{
			tfc.NewArgsBuilder().
				WithTradeArgs(colors[a1.OrgID], colors[a2.OrgID], tfcPb.Resource_HILL, 2).
				Args(),
			tfc.NewArgsBuilder().
				WithTradeArgs(colors[a2.OrgID], colors[a1.OrgID], tfcPb.Resource_HILL, 2).
				Args(),
		}

		eOut <- makeAlliance(gameName, allianceUUID, allies, terms...)

	}
}

type drmItem struct {
	Author     string
	CreateTime time.Time
	Info       string
	Item       []byte
}

func scriptDRM() []drmItem {
	rand.Seed(time.Now().Unix())

	items := []drmItem{}
	for i := 0; i < 30; i++ {
		payload := make([]byte, 8)
		rand.Read(payload)
		items = append(items, drmItem{
			Author:     "foo" + strconv.Itoa(i),
			CreateTime: time.Now(),
			Info:       "",
			Item:       payload,
		})
	}

	return items
}

func execDRMAsync(gameName string, respChan chan (bool), orgsIn chan ([]string), orgsOut chan ([]string)) {

	ccReq := resmgmt.InstantiateCCRequest{
		Name:    "drm",
		Path:    "contract/fabric/drm",
		Version: "1.0",
	}

	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs[:2], ccReq)
	orgsOut <- orgs
	defer closePlayers(players)

	tttScript1 := scriptDRM()
	_, err = runScriptDRM(tttScript1, "drm", players)
	if err != nil {
		panic(err)
	}

	log.Printf("Finished running test.")

	respChan <- true
}

func runScriptDRM(script []drmItem, ccName string, players []*TFCClient) ([]channel.Response, error) {
	responses := make([]channel.Response, len(script))
	for i := range script {
		msg := script[i]
		log.Printf("Executing script step %v", msg)
		trxArgs, err := json.Marshal(msg)
		if err != nil {
			return responses, err
		}

		pID := i % len(players)
		r, err := invokeAndMeasure(players[pID], ccName, trxArgs)
		if err != nil {
			return responses, err
		}
		responses[i] = r
	}

	return responses, nil
}

func execTTTGameAsync(gameName string, errOut chan (error), orgsIn chan ([]string), orgsOut chan ([]string)) {

	defer recordFailure()

	ccReq := resmgmt.InstantiateCCRequest{
		Name:    "ttt",
		Path:    "github.com/stefanprisca/strategy-code/tictactoe",
		Version: "1.0",
	}

	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs[:2], ccReq)
	orgsOut <- orgs

	if err != nil {
		errOut <- err
		panic(err)
	}

	defer closePlayers(players)

	tttScript1 := scriptTTT1(players[0], players[1])
	_, err = runGameScript(tttScript1, "ttt", players)
	if err != nil {
		errOut <- err
		panic(err)
	}
	log.Printf("Finished running test.")

	errOut <- nil
}

func execTFCGameAsync(gameName string, errOut chan (error), orgsIn chan ([]string), orgsOut chan ([]string)) {

	defer recordFailure()

	ccReq := resmgmt.InstantiateCCRequest{
		Name:    "tfc",
		Path:    "github.com/stefanprisca/strategy-code/cmd/tfc",
		Version: "1.0",
	}

	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs, ccReq)
	orgsOut <- orgs

	if err != nil {
		errOut <- err
		panic(err)
	}

	defer closePlayers(players)

	// Instantiate the alliance CC

	ccReq = resmgmt.InstantiateCCRequest{
		Name:    "alliance",
		Path:    "github.com/stefanprisca/strategy-code/cmd/alliance",
		Version: "1.0",
	}
	err = runChaincode(players, ccReq, gameName, [][]byte{})

	if err != nil {
		errOut <- err
		panic(err)
	}

	tfcScript, alGenerator := scriptTFC1(players[0], players[1], players[2])
	// _, err = runGameScript(tfcScript, "tfc", players)
	// if err != nil {
	// 	errOut <- err
	// 	panic(err)
	// }
	allianceErrOut := make(chan (error), len(tfcScript))

	j := 0
	stepSize := 12
	for i := 0; i < len(tfcScript); i += stepSize {
		_, err = runGameScript(tfcScript[j:i], "tfc", players)
		if err != nil {
			errOut <- err
			panic(err)
		}

		go alGenerator(i, gameName, allianceErrOut)
		j = i
	}

	log.Printf("Finished running test.")

	for ; j >= 0; j -= stepSize {
		log.Printf("Waiting for alliances to create...%d", j)
		err = <-allianceErrOut
		if err != nil {

			errOut <- err
			panic(err)
		}
	}

	errOut <- nil
}

func runGameScript(script []scriptStep, ccName string, players []*TFCClient) ([]channel.Response, error) {
	responses := make([]channel.Response, len(script))
	for i := 0; i < len(script); i++ {
		msg := script[i].message
		player := script[i].player
		log.Printf("Executing script step %v", msg)
		trxArgs, err := proto.Marshal(msg)
		if err != nil {
			return responses, err
		}

		ms := rand.Intn(100) + 100
		stepInterval, _ := time.ParseDuration(fmt.Sprintf("%vms", ms))
		time.Sleep(stepInterval)

		r, err := invokeAndMeasure(player, ccName, trxArgs)

		if err != nil {
			log.Println(err.Error())
			continue
		}

		if gcArgs, ok := script[i].message.(*tfcPb.GameContractTrxArgs); ok {

			for _, ccReg := range player.GameObservers {
				if ccReg.terminated {
					continue
				}

				notifyArgs := &tfcPb.TrxCompletedArgs{
					CompletedTrxArgs: gcArgs,
				}
				ccReg.TrxComplete <- notifyArgs
			}
		}

		responses[i] = r
	}

	return responses, nil
}
