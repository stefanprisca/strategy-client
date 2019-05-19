package tfc

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
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

func scriptTFC1(p1, p2, p3 *TFCClient) []scriptStep {

	p1C, p2C, p3C := tfcPb.Player_RED, tfcPb.Player_GREEN, tfcPb.Player_BLUE

	s := []scriptStep{
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p1C).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p2C).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithJoinArgs(p3C).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p1C, p2C, tfcPb.Resource_CAMP, 2).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p1C, p3C, tfcPb.Resource_HILL, -2).Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p1},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p2C, p1C, tfcPb.Resource_CAMP, 2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p2C, p3C, tfcPb.Resource_PASTURE, -2).Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p2},
		{message: tfcCC.NewArgsBuilder().WithRollArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p1C, tfcPb.Resource_HILL, -2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithTradeArgs(p3C, p2C, tfcPb.Resource_PASTURE, -2).Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
		{message: tfcCC.NewArgsBuilder().WithNextArgs().Args(), player: p3},
	}

	for i := 0; i < 5; i++ {
		s = append(s, s[3:]...)
	}

	return s
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

	tfcScript := scriptTFC1(players[0], players[1], players[2])
	_, err = runGameScript(tfcScript, gameName, players, "TFC")
	if err != nil {
		panic(err)
	}
	log.Printf("Finished running test.")

	respChan <- true
}

func runGameScript(script []scriptStep, chanName string, players []*TFCClient, ccName string) ([]channel.Response, error) {
	responses := make([]channel.Response, len(script))
	for i := range script {
		msg := script[i].message
		player := script[i].player
		log.Printf("Executing script step %v", msg)
		trxArgs, err := proto.Marshal(msg)
		if err != nil {
			return responses, err
		}

		r, err := invokeAndMeasure(player, chanName, trxArgs, ccName)
		if err != nil {
			return responses, err
		}
		responses[i] = r
	}

	return responses, nil
}

func invokeAndMeasure(player *TFCClient, chanName string, trxArgs []byte, ccName string) (channel.Response, error) {

	st := time.Now()
	r, err := invokeGameChaincode(player, chanName, trxArgs)
	if err != nil {
		return r, err
	}

	rt := time.Since(st).Seconds()
	player.Metrics.
		With(CCLabel, ccName).
		Observe(rt)

	// ms := rand.Intn(1000) + 500
	// stepInterval, _ := time.ParseDuration(fmt.Sprintf("%vms", ms))
	// time.Sleep(stepInterval)
	return r, nil
}
