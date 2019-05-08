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
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"text/template"

	pPrint "github.com/stefanprisca/strategy-code/prettyprint"
	"github.com/stefanprisca/strategy-code/tfc"

	"github.com/golang/protobuf/proto"

	//mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"

	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	tfcPb "github.com/stefanprisca/strategy-protobufs/tfc"
	tttPb "github.com/stefanprisca/strategy-protobufs/tictactoe"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"

	// "github.com/hyperledger/fabric-sdk-go/test/integration"

	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"

	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

var (
	scfixturesPath = path.Join(os.Getenv("SCFIXTURES"), "tfc")
	gopath         = os.Getenv("GOPATH")
	ccPath         = "github.com/stefanprisca/strategy-code/cmd/tfc"
)

type ccDescriptor struct {
	ccID      string
	ccPath    string
	ccVersion string
	ccPackage *resource.CCPackage
}

/*
	Test that will
	1) Create the sdk
	2) Connect both players
	3) Play a game
*/
func TestE2E(t *testing.T) {

	gameName := "tfc5"
	chanOrgs := []string{Player1, Player2, Player3}

	cfgPath, err := generateChannelArtifacts(gameName, chanOrgs)
	require.NoError(t, err)

	players, err := generatePlayers(cfgPath, chanOrgs)
	require.NoError(t, err)
	defer closePlayers(players)

	err = startGame(players, cfgPath, gameName)
	require.NoError(t, err)

	err = invokeChaincodeTFC(players[0], gameName)
	require.NoError(t, err)
}

func generateChannelArtifacts(channelName string, chanOrgs []string) (string, error) {
	/*
		1) Fill out chan template
		2) Generate using configtex tool
		3) Submit chan transaction
		4) Join channel.
	*/
	cfgPath := path.Join(scfixturesPath, "temp", channelName)
	err := os.MkdirAll(cfgPath, 0777)
	if err != nil {
		return "", fmt.Errorf("Could not create config path. %s", err)
	}

	cfgFilePath := path.Join(cfgPath, "configtx.yaml")
	err = executeTemplate(cfgFilePath, "configtx.yaml_template", chanOrgs)
	if err != nil {
		return "", fmt.Errorf("could not generate channel cfg: %s", err)
	}

	genScriptPath := path.Join(cfgPath, "generateChan.sh")
	err = executeTemplate(genScriptPath, "generateChan.sh_template", chanOrgs)
	if err != nil {
		return "", fmt.Errorf("could not generate channel cfg: %s", err)
	}

	os.Setenv("FABRIC_CFG_PATH", cfgPath)
	os.Setenv("CHANNEL_NAME", channelName)
	fmt.Println(cfgPath)

	changen := exec.Command("/bin/sh", genScriptPath, channelName)
	result, err := changen.CombinedOutput()
	print(result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute commad. %s \n %s", err.Error(), string(result))
	}

	return cfgPath, nil
}

func generatePlayers(cfgPath string, chanOrgs []string) ([]*TFCClient, error) {

	players := []*TFCClient{}
	lowCapOrgs := []string{}
	for _, org := range chanOrgs {
		lowCapOrgs = append(lowCapOrgs, strings.ToLower(org))
	}

	for i, org := range chanOrgs {
		cfgName := org + "Config.yaml"
		clientCfg := path.Join(cfgPath, cfgName)
		tmplName := "p" + strconv.Itoa(i) + "Config.yaml_template"
		err := executeTemplate(clientCfg, tmplName, lowCapOrgs)
		if err != nil {
			return nil, fmt.Errorf("could not create client cfg: %s", err)
		}

		c, err := NewTFCClient(cfgPath, clientCfg, org)
		if err != nil {
			return nil, fmt.Errorf("could not create new client: %s", err)
		}
		players = append(players, c)
	}

	return players, nil
}

func closePlayers(players []*TFCClient) {
	for _, p := range players {
		p.SDK.Close()
	}
}

func executeTemplate(filePath, tplName string, chanOrgs []string) error {

	resultFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Could not create file. %s", err)
	}
	defer resultFile.Close()

	tplPath := path.Join("templates", "fabric", tplName)
	tmpl, err := template.ParseFiles(tplPath)
	if err != nil {
		return fmt.Errorf("Could not load template. %s", err)
	}
	tmpl.Execute(resultFile, chanOrgs)
	return nil
}

func startGame(players []*TFCClient, chanCfg, chanName string) error {
	chanTxPath := path.Join(chanCfg, chanName+".tx")

	// Create the game channel
	p1 := players[0]
	signatures := getSignatures(players)
	err := createChannel(p1, signatures, chanName, chanTxPath)
	if err != nil {
		return fmt.Errorf("could not create game channel: %s", err)
	}

	// join all the peers to the channel
	for _, p := range players {
		err = joinGame(p, chanName)
		if err != nil {
			return fmt.Errorf("could not join game channel: %s", err)
		}
		err = updateAnchorPeers(p, chanName)
		if err != nil {
			return fmt.Errorf("could not update anchor peers: %s", err)
		}
	}

	ccPkg, err := createCC(ccPath)
	if err != nil {
		return fmt.Errorf("could not create cc package: %s", err)
	}

	// Install game chaincode to the peers
	ccReq := resmgmt.InstallCCRequest{
		Name:    chanName,
		Path:    ccPath,
		Version: "0.1.0",
		Package: ccPkg}

	ccPolicyString := fmt.Sprintf("OR('%s', '%s', '%s')",
		players[0].Endorser, players[1].Endorser, players[2].Endorser)

	log.Printf("Created policy string: %s", ccPolicyString)

	ccPolicy, err := cauthdsl.FromString(ccPolicyString)
	if err != nil {
		return fmt.Errorf("could not create cc policy: %s", err)
	}

	err = deployChaincode(players, ccReq, ccPolicy, chanName)
	if err != nil {
		return fmt.Errorf("could not install cc: %s", err)
	}
	return nil
}

func getSignatures(players []*TFCClient) []msp.SigningIdentity {
	result := []msp.SigningIdentity{}
	for _, p := range players {
		result = append(result, p.SigningIdentity)
	}
	return result
}

func createChannel(player *TFCClient, signatures []msp.SigningIdentity, chanName, chanTxPath string) error {

	r, err := os.Open(chanTxPath)
	if err != nil {
		return fmt.Errorf("failed to open channel config: %s", err)
	}
	defer r.Close()

	orgResMgmt := player.ResMgmt
	resp, err := orgResMgmt.SaveChannel(
		resmgmt.SaveChannelRequest{
			ChannelID:         chanName,
			ChannelConfig:     r,
			SigningIdentities: signatures,
		},
		resmgmt.WithOrdererEndpoint(OrdererEndpoint),
		resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return fmt.Errorf("failed to save channel: %s", err)
	}

	if resp.TransactionID == "" {
		return fmt.Errorf("Failed to save channel")
	}

	return nil
}

func joinGame(player *TFCClient, chanName string) error {
	log.Printf("Joining channel for peer %s channel: %s", player.OrgID, chanName)

	orgResMgmt := player.ResMgmt
	// Org peers join channel
	if err := orgResMgmt.JoinChannel(chanName,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(OrdererEndpoint)); err != nil {
		return fmt.Errorf("Org %s peers failed to JoinChannel: %s", player.OrgID, err)
	}
	return nil
}

func updateAnchorPeers(player *TFCClient, chanName string) error {
	log.Printf("Updating anchor peers for %s channel: %s", player.OrgID, chanName)
	signs := []msp.SigningIdentity{player.SigningIdentity}
	req := resmgmt.SaveChannelRequest{
		ChannelID:         chanName,
		ChannelConfigPath: player.AnchorPeerConfigFile,
		SigningIdentities: signs,
	}

	orgResMgmt := player.ResMgmt
	tx, err := orgResMgmt.SaveChannel(req,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(OrdererEndpoint))
	if err != nil {
		return fmt.Errorf("Anchor peers failed to update for channel: %s", err)
	}

	if tx.TransactionID == "" {
		return fmt.Errorf("Failed to save channel")
	}

	return nil
}

func createCC(ccPath string) (*resource.CCPackage, error) {
	ccPkg, err := packager.NewCCPackage(ccPath, gopath)
	if err != nil {
		return nil, fmt.Errorf("could not create cc package: %s", err)
	}

	return ccPkg, nil
}

func deployChaincode(players []*TFCClient, ccReq resmgmt.InstallCCRequest,
	ccPolicy *common.SignaturePolicyEnvelope, chanName string) error {

	for _, player := range players {
		orgResMgmt := player.ResMgmt
		log.Printf("Installing chaincode %s for %s channel: %s", ccReq.Name, player.OrgID, chanName)
		_, err := orgResMgmt.InstallCC(ccReq,
			resmgmt.WithRetry(retry.DefaultResMgmtOpts))
		if err != nil {
			return fmt.Errorf("failed to install cc: %s", err)
		}
	}

	p1 := players[0]
	log.Printf("Instantiating chaincode %s for %s on channel %s with policy %s",
		ccReq.Name, p1.OrgID, chanName, ccPolicy)
	// Org resource manager will instantiate 'example_cc' on channel
	_, err := p1.ResMgmt.InstantiateCC(
		chanName,
		resmgmt.InstantiateCCRequest{
			Name:    ccReq.Name,
			Path:    ccReq.Path,
			Version: ccReq.Version,
			Args:    [][]byte{},
			Policy:  ccPolicy,
		},
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
	)

	if err != nil {
		return fmt.Errorf("failed to instantiate cc: %s", err)
	}

	return err
}

func invokeChaincodeTFC(player *TFCClient, chanName string) error {

	// Org resource management client
	orgResMgmt := player.ResMgmt
	log.Printf("Invoking game chaincode for channel %s", chanName)
	ccResp, err := orgResMgmt.QueryInstantiatedChaincodes(chanName)
	if err != nil {
		return fmt.Errorf("could not get chaincodes: %s", err)
	}
	log.Println("Got the chaincodes installed", ccResp.Chaincodes)

	clientChannelContext := player.SDK.ChannelContext(chanName,
		fabsdk.WithUser(User),
		fabsdk.WithOrg(player.OrgID))

	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return fmt.Errorf("could not get channel client: %s", err)
	}

	log.Printf("Connected client for %s\n", player.OrgID)

	trxBytes, err := tfc.NewArgsBuilder().
		WithJoinArgs(tfcPb.Player_RED).
		Build()

	if err != nil {
		return fmt.Errorf("could not marshal trx args: %s", err)
	}
	fmt.Println(trxBytes)

	response, err := client.Execute(
		channel.Request{
			ChaincodeID: chanName,
			Fcn:         "move",
			Args:        trxBytes},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return fmt.Errorf("Failed to invoke cc: %s", err)
	}
	fmt.Println("Issued chaincode invoke.")

	gdataBytes := response.Payload
	gdata := &tfcPb.GameData{}
	err = proto.Unmarshal(gdataBytes, gdata)
	if err != nil {
		return fmt.Errorf("could not unmarshal response from Tictactoe: %s", err)
	}

	canvas := pPrint.NewTFCBoardCanvas().
		PrettyPrintTfcBoard(*gdata.GetBoard())
	fmt.Println(canvas)

	return nil
}

func invokeChaincodeTTT(player *TFCClient, chanName string) error {

	// Org resource management client
	orgResMgmt := player.ResMgmt
	log.Printf("Invoking game chaincode for channel %s", chanName)
	ccResp, err := orgResMgmt.QueryInstantiatedChaincodes(chanName)
	if err != nil {
		return fmt.Errorf("could not get chaincodes: %s", err)
	}
	log.Println("Got the chaincodes installed", ccResp.Chaincodes)

	clientChannelContext := player.SDK.ChannelContext(chanName,
		fabsdk.WithUser(User),
		fabsdk.WithOrg(player.OrgID))

	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return fmt.Errorf("could not get channel client: %s", err)
	}

	log.Printf("Connected client for %s\n", player.OrgID)

	mvPayload := &tttPb.MoveTrxPayload{Mark: tttPb.Mark_X, Position: 3}
	trxArgs := &tttPb.TrxArgs{Type: tttPb.TrxType_MOVE, MovePayload: mvPayload}

	trxBytes, err := proto.Marshal(trxArgs)
	if err != nil {
		return fmt.Errorf("could not marshal trx args: %s", err)
	}
	fmt.Println(trxBytes)

	response, err := client.Execute(
		channel.Request{
			ChaincodeID: chanName,
			Fcn:         "dummy",
			Args:        [][]byte{trxBytes}},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return fmt.Errorf("Failed to invoke cc: %s", err)
	}
	fmt.Println("Issued chaincode invoke.")

	gBoardBytes := response.Payload
	gBoard := &tttPb.TttContract{}
	err = proto.Unmarshal(gBoardBytes, gBoard)
	if err != nil {
		return fmt.Errorf("could not unmarshal response from Tictactoe: %s", err)
	}
	fmt.Println(gBoard.GetPositions())

	return nil
}
