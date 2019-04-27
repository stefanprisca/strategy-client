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
	"testing"
	"text/template"

	"github.com/golang/protobuf/proto"

	//mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"

	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	tttPf "github.com/stefanprisca/strategy-protobufs/tictactoe"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"

	// "github.com/hyperledger/fabric-sdk-go/test/integration"

	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

var (
	scfixturesPath = path.Join(os.Getenv("SCFIXTURES"), "tfc")
	gopath         = os.Getenv("GOPATH")
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

	gameName := "newchan"
	chanOrgs := []string{Org1, Org2, Org3}

	chanCfg, err := generateChannelArtifacts(gameName, chanOrgs)
	require.NoError(t, err)

	players := []*TFCClient{}

	for _, org := range chanOrgs {
		clientCfg := path.Join("config", org+"Config.yaml")
		c, err := NewTFCClient(chanCfg, clientCfg, org)
		require.NoError(t, err)
		defer c.SDK.Close()
		players = append(players, c)
	}

	err = startGame(players, chanCfg, gameName)
	require.NoError(t, err)

	// err = invokeChaincode(sdk, org1, chanName)
	// require.NoError(t, err)
}

func generateChannelArtifacts(channelName string, chanOrgs []string) (string, error) {
	/*
		1) Fill out chan template
		2) Generate using configtex tool
		3) Submit chan transaction
		4) Join channel.
	*/

	chanTemplate, err := template.ParseFiles("templates/fabric/configtx.yaml_template")
	if err != nil {
		return "", fmt.Errorf("Could not load channel template. %s", err)
	}
	cfgPath := path.Join(scfixturesPath, "temp", channelName)
	err = os.MkdirAll(cfgPath, 0777)
	if err != nil {
		return "", fmt.Errorf("Could not create config path. %s", err)
	}

	cfgFilePath := path.Join(cfgPath, "configtx.yaml")
	cfgFile, err := os.Create(cfgFilePath)
	if err != nil {
		return "", fmt.Errorf("Could not create cfg file. %s", err)
	}

	defer cfgFile.Close()

	chanTemplate.Execute(cfgFile, chanOrgs)

	os.Setenv("FABRIC_CFG_PATH", cfgPath)
	os.Setenv("CHANNEL_NAME", channelName)
	fmt.Println(cfgPath)

	changen := exec.Command("/bin/sh", "scripts/generateChan.sh", channelName)
	result, err := changen.CombinedOutput()
	print(result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute commad. %s \n %s", err.Error(), string(result))
	}

	return cfgPath, nil
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

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
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

	ccPolicy := cauthdsl.AcceptAllPolicy
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
	log.Printf("Instantiating chaincode %s for %s channel: %s", ccReq.Name, p1.OrgID, chanName)
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

func invokeChaincode(sdk *fabsdk.FabricSDK, org, chanName string) error {

	adminContext := sdk.Context(fabsdk.WithUser(AdminUser), fabsdk.WithOrg(org))

	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	ccResp, err := orgResMgmt.QueryInstantiatedChaincodes(chanName)
	if err != nil {
		return fmt.Errorf("could not get chaincodes: %s", err)
	}
	fmt.Println("Got the chaincodes installed", ccResp.Chaincodes)

	clientChannelContext := sdk.ChannelContext(chanName, fabsdk.WithUser(User), fabsdk.WithOrg(org))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return fmt.Errorf("could not get channel client: %s", err)
	}

	fmt.Printf("Connected client for %s\n", org)

	mvPayload := &tttPf.MoveTrxPayload{Mark: tttPf.Mark_O, Position: 3}
	trxArgs := &tttPf.TrxArgs{Type: tttPf.TrxType_MOVE, MovePayload: mvPayload}

	trxBytes, err := proto.Marshal(trxArgs)
	if err != nil {
		return fmt.Errorf("could not marshal trx args: %s", err)
	}
	fmt.Println(trxBytes)

	response, err := client.Execute(
		channel.Request{
			ChaincodeID: chanName,
			Fcn:         "move",
			Args:        [][]byte{trxBytes}},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return fmt.Errorf("Failed to invoke cc: %s", err)
	}
	fmt.Println("Issued chaincode invoke.")

	gBoardBytes := response.Payload
	gBoard := &tttPf.TttContract{}
	err = proto.Unmarshal(gBoardBytes, gBoard)
	if err != nil {
		return fmt.Errorf("could not unmarshal response from Tictactoe: %s", err)
	}
	fmt.Println(gBoard.GetPositions())

	return nil
}

func makeSDK(clientCfg string) (*fabsdk.FabricSDK, error) {

	configOpt := config.FromFile(clientCfg)
	sdk, err := fabsdk.New(configOpt)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new SDK: %s", err)
	}
	return sdk, nil
}
