package tfc

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/stefanprisca/strategy-code/tfc"
	tfcCC "github.com/stefanprisca/strategy-code/tfc"
	tfcPb "github.com/stefanprisca/strategy-protobufs/tfc"
	tttPb "github.com/stefanprisca/strategy-protobufs/tictactoe"
)

type asyncExecutor = func(gameName string, respChan chan (bool), orgsIn chan ([]string), orgsOut chan ([]string))

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
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p1C, tfcPb.Resource_HILL, 2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p2C, tfcPb.Resource_HILL, 2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
	}

	for i := 0; i < 5; i++ {
		s = append(s, s[3:]...)
	}

	return s, func(i int, gameName string, eOut chan error) {
		a1 := s[i%3].player
		a2 := s[(i+1)%3].player

		allies := []*TFCClient{a1, a2}
		allianceUUID := uint32(100 + i)

		terms := []*tfcPb.GameContractTrxArgs{
			tfc.NewArgsBuilder().
				WithTradeArgs(colors[a1.OrgID], colors[a2.OrgID], tfcPb.Resource_HILL, 2).
				Args(),
			tfc.NewArgsBuilder().
				WithTradeArgs(colors[a2.OrgID], colors[a1.OrgID], tfcPb.Resource_HILL, 2).
				Args(),
		}

		for i := 0; i < 3; i++ {
			terms = append(terms, terms...)
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

	ccPath := "contract/fabric/drm"
	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs[:2], ccPath)
	orgsOut <- orgs
	defer closePlayers(players)

	tttScript1 := scriptDRM()
	_, err = runScriptDRM(tttScript1, gameName, players)
	if err != nil {
		panic(err)
	}

	log.Printf("Finished running test.")

	respChan <- true
}

func runScriptDRM(script []drmItem, chanName string, players []*TFCClient) ([]channel.Response, error) {
	responses := make([]channel.Response, len(script))
	for i := range script {
		msg := script[i]
		log.Printf("Executing script step %v", msg)
		trxArgs, err := json.Marshal(msg)
		if err != nil {
			return responses, err
		}

		pID := i % len(players)
		r, err := invokeAndMeasure(players[pID], chanName, trxArgs, "DRM")
		if err != nil {
			return responses, err
		}
		responses[i] = r
	}

	return responses, nil
}

func execTTTGameAsync(gameName string, respChan chan (bool), orgsIn chan ([]string), orgsOut chan ([]string)) {

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"

	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs[:2], ccPath)
	orgsOut <- orgs

	if err != nil {
		panic(err)
	}

	defer closePlayers(players)

	tttScript1 := scriptTTT1(players[0], players[1])
	_, err = runGameScript(tttScript1, gameName, players, "TTT")
	if err != nil {
		panic(err)
	}
	log.Printf("Finished running test.")

	respChan <- true
}

func execTFCGameAsync(gameName string, respChan chan (bool), orgsIn chan ([]string), orgsOut chan ([]string)) {

	ccPath := "github.com/stefanprisca/strategy-code/cmd/tfc"

	orgs := <-orgsIn
	players, err := bootstrapChannel(gameName, orgs, ccPath)
	orgsOut <- orgs

	if err != nil {
		panic(err)
	}

	defer closePlayers(players)
	tfcScript, alGenerator := scriptTFC1(players[0], players[1], players[2])

	allianceErrOut := make(chan (error), len(tfcScript))

	j := 0
	for i := 0; i < len(tfcScript); i += 20 {
		_, err = runGameScript(tfcScript[j:i], gameName, players, "TFC")
		if err != nil {
			panic(err)
		}

		go alGenerator(i, gameName, allianceErrOut)
		j = i
	}

	log.Printf("Finished running test.")

	select {
	case err = <-allianceErrOut:
		if err != nil {
			panic(err)
		}
	default:
	}

	respChan <- true
}

func runGameScript(script []scriptStep, chanName string, players []*TFCClient, ccName string) ([]channel.Response, error) {
	responses := make([]channel.Response, len(script))
	for i := 0; i < len(script); i++ {
		msg := script[i].message
		player := script[i].player
		log.Printf("Executing script step %v", msg)
		trxArgs, err := proto.Marshal(msg)
		if err != nil {
			return responses, err
		}

		r, err := invokeAndMeasure(player, chanName, trxArgs, ccName)

		if err != nil {
			log.Println(err.Error())
			i--
			continue
		}

		if gcArgs, ok := script[i].message.(*tfcPb.GameContractTrxArgs); ok {

			for _, ccReg := range player.GameObservers {
				if ccReg.terminated {
					continue
				}

				notifyArgs := &tfcPb.TrxCompletedArgs{
					CompletedTrxArgs: gcArgs,
					ObserverID:       ccReg.UUID,
				}
				ccReg.TrxComplete <- notifyArgs
			}
		}

		responses[i] = r
	}

	return responses, nil
}
