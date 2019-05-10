package tfc

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
)

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

type pConfigData struct {
	For  string
	Orgs []string
}

func generatePlayers(cfgPath string, chanOrgs []string, gameName string) ([]*TFCClient, error) {

	players := []*TFCClient{}
	lowCapOrgs := []string{}
	for _, org := range chanOrgs {
		lowCapOrgs = append(lowCapOrgs, strings.ToLower(org))
	}

	for i, org := range chanOrgs {
		cfgName := org + "Config.yaml"
		clientCfg := path.Join(cfgPath, cfgName)

		tplName := "pConfig.yaml_template"
		tlpData := pConfigData{lowCapOrgs[i], lowCapOrgs}
		err := executeTemplate(clientCfg, tplName, tlpData)
		if err != nil {
			return nil, fmt.Errorf("could not create client cfg: %s", err)
		}

		c, err := NewTFCClient(cfgPath, clientCfg, org, gameName)
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

func executeTemplate(filePath, tplName string, data interface{}) error {

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
	tmpl.Execute(resultFile, data)
	return nil
}

func startGame(players []*TFCClient, chanCfg, ccPath, chanName string) error {
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

		err = updateChannelClient(p, chanName)
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

	endorsers := []string{}
	for _, p := range players {
		endorsers = append(endorsers, fmt.Sprintf("'%s'", p.Endorser))
	}

	ccPolicyString := fmt.Sprintf("OR(%s)", strings.Join(endorsers, ", "))

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

func updateChannelClient(p *TFCClient, gameName string) error {

	clientChannelContext := p.SDK.ChannelContext(gameName,
		fabsdk.WithUser(User),
		fabsdk.WithOrg(p.OrgID))

	// Channel client is used to query and execute transactions (Org1 is default org)
	chanClient, err := channel.New(clientChannelContext)
	if err != nil {
		return fmt.Errorf("could not get channel client: %s", err)
	}

	p.ChannelClient = chanClient

	return nil
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

func invokeGameChaincode(player *TFCClient, chanName string, protoArgs []byte) (channel.Response, error) {

	// Org resource management client
	// orgResMgmt := player.ResMgmt
	log.Printf("Invoking game chaincode for client %v on channel %s", player, chanName)
	// ccResp, err := orgResMgmt.QueryInstantiatedChaincodes(chanName)
	// if err != nil {
	// 	return channel.Response{}, fmt.Errorf("could not get chaincodes: %s", err)
	// }
	// log.Println("Got the chaincodes installed", ccResp.Chaincodes)

	// log.Printf("Connected client for %s\n", player.OrgID)
	response, err := player.ChannelClient.Execute(
		channel.Request{
			ChaincodeID: chanName,
			Fcn:         "move",
			Args:        [][]byte{protoArgs}},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return channel.Response{}, fmt.Errorf("Failed to invoke cc: %s", err)
	}
	// gdataBytes := response.Payload
	// gdata := &tfcPb.GameData{}
	// err = proto.Unmarshal(gdataBytes, gdata)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not unmarshal response from Tictactoe: %s", err)
	// }

	// canvas := pPrint.NewTFCBoardCanvas().
	// 	PrettyPrintTfcBoard(*gdata.GetBoard())
	// fmt.Println(canvas)

	return response, nil
}
