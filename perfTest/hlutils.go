package tfc

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	tfcPb "github.com/stefanprisca/strategy-protobufs/tfc"
)

func bootstrapAndMeasureChannel(gameName string, chanOrgs []string, ccReq resmgmt.InstantiateCCRequest) ([]*TFCClient, error) {

	// Observe a 0 to boot the ops measurement
	GetPlayerMetrics().
		With(CCLabel, "Operations").
		With(CCFailedLabel, "False").
		Observe(0)

	st := time.Now()
	p, err := bootstrapChannel(gameName, chanOrgs, ccReq)
	rt := time.Since(st).Seconds()

	if err != nil {
		return p, err
	}

	GetPlayerMetrics().
		With(CCLabel, "Operations").
		With(CCFailedLabel, "False").
		Observe(rt)
	return p, err
}

func bootstrapChannel(gameName string, chanOrgs []string, ccReq resmgmt.InstantiateCCRequest) ([]*TFCClient, error) {

	cfgPath, err := generateChannelArtifacts(gameName, chanOrgs)
	if err != nil {
		return nil, err
	}

	players, err := generatePlayers(cfgPath, chanOrgs, gameName)
	if err != nil {
		return nil, err
	}
	for _, p := range players {
		p.Metrics = GetPlayerMetrics()
	}

	// ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
	// os.Setenv("GOPATH", "/home/stefan/workspace/hyperledger/caliper/packages/caliper-application")
	//ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
	err = startGame(players, cfgPath, gameName, ccReq)
	if err != nil {
		return nil, err
	}
	return players, nil
}

type genScriptData struct {
	Orgs          []string
	FabricCfgPath string
	ChannelName   string
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
	genScriptData := genScriptData{chanOrgs, cfgPath, channelName}
	err = executeTemplate(genScriptPath, "generateChan.sh_template", genScriptData)
	if err != nil {
		return "", fmt.Errorf("could not generate channel cfg: %s", err)
	}

	fmt.Println(cfgPath)

	changen := exec.Command("/bin/sh", genScriptPath, channelName)

	changen.Env = os.Environ()
	changen.Env = append(changen.Env, "FABRIC_CFG_PATH="+cfgPath)
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
		for _, ccReg := range p.GameObservers {
			if ccReg.terminated {
				continue
			}
			ccReg.Shutdown <- true
		}
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

func startGame(players []*TFCClient, chanCfg, chanName string, ccReq resmgmt.InstantiateCCRequest) error {
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

	return runChaincode(players, ccReq, chanName, [][]byte{})
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

func deployChaincode(ccPath, name string,
	players []*TFCClient) (*resmgmt.InstallCCRequest, error) {

	ccPkg, err := createCC(ccPath)
	if err != nil {
		return nil, fmt.Errorf("could not create cc package: %s", err)
	}

	// Install game chaincode to the peers
	ccReq := &resmgmt.InstallCCRequest{
		Name:    name,
		Path:    ccPath,
		Version: "1.0",
		Package: ccPkg}

	if err != nil {
		return nil, fmt.Errorf("could not create cc policy: %s", err)
	}

	for _, player := range players {
		orgResMgmt := player.ResMgmt
		log.Printf("Installing chaincode %s for %s", ccReq.Name, player.OrgID)
		_, err := orgResMgmt.InstallCC(*ccReq,
			resmgmt.WithRetry(retry.DefaultResMgmtOpts),
			resmgmt.WithTargetEndpoints(player.PeerEndpoint))
		if err != nil {
			return nil, fmt.Errorf("failed to install cc: %s", err)
		}
	}

	return ccReq, nil
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

func runChaincode(players []*TFCClient,
	ccReq resmgmt.InstantiateCCRequest,
	chanName string,
	initArgs [][]byte) error {

	endorsers := []string{}
	for _, p := range players {
		endorsers = append(endorsers, fmt.Sprintf("'%s'", p.Endorser))
	}

	ccPolicyString := fmt.Sprintf("OutOf(2, %s)", strings.Join(endorsers, ", "))
	log.Printf("Created policy string: %s", ccPolicyString)

	ccPolicy, err := cauthdsl.FromString(ccPolicyString)

	p1 := players[0]
	log.Printf("Instantiating chaincode %s for %s on channel %s with policy %s",
		ccReq.Name, p1.OrgID, chanName, ccPolicy)
	// Org resource manager will instantiate 'example_cc' on channel

	teps := make([]string, len(players))
	for i, p := range players {
		teps[i] = p.PeerEndpoint
	}

	colDefinitions, err := getCollectionDefinitions(players)
	if err != nil {
		return fmt.Errorf("failed to create collection definitions: %v", err)
	}

	_, err = p1.ResMgmt.InstantiateCC(
		chanName,
		resmgmt.InstantiateCCRequest{
			Name:       ccReq.Name,
			Path:       ccReq.Path,
			Version:    ccReq.Version,
			Args:       initArgs,
			Policy:     ccPolicy,
			CollConfig: colDefinitions,
		},
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithTargetEndpoints(teps...),
	)

	if err != nil {
		return fmt.Errorf("failed to instantiate cc: %s", err)
	}

	return err
}

func makeAndMeasureAlliance(gameName string, allianceUUID uint32, allies []*ally, terms ...*tfcPb.GameContractTrxArgs) error {

	st := time.Now()
	err := makeAlliance(gameName, allianceUUID, allies, terms...)
	rt := time.Since(st).Seconds()

	if err != nil {
		GetPlayerMetrics().
			With(CCLabel, "Operations").
			With(CCFailedLabel, "True").
			Observe(rt)
		return err
	}

	GetPlayerMetrics().
		With(CCLabel, "Operations").
		With(CCFailedLabel, "False").
		Observe(rt)

	return err
}

func makeAlliance(gameName string, allianceUUID uint32, allies []*ally, terms ...*tfcPb.GameContractTrxArgs) error {

	allianceCCPath := "local-cc/alliance"
	log.Printf("Creating alliance for players %v %v...", allies[0].OrgID, allies[1].OrgID)
	allianceName := gameName + fmt.Sprintf("%d", allianceUUID)
	players := []*TFCClient{allies[0].TFCClient, allies[1].TFCClient}
	// Alliance still needs to be deployed as a specific CC.
	// Endorsment policies don't work otheriwse
	_, err := deployChaincode(allianceCCPath, allianceName, players)
	if err != nil {
		return err
	}

	// Instantiate the alliance CC

	ccReq := resmgmt.InstantiateCCRequest{
		Name:    allianceName,
		Path:    "github.com/stefanprisca/strategy-code/cmd/alliance",
		Version: "1.0",
	}
	err = runChaincode(players, ccReq, gameName, [][]byte{})
	if err != nil {
		return err
	}

	ad := &tfcPb.AllianceData{
		Lifespan:       3,
		StartGameState: tfcPb.GameState_RTRADE,
		Terms:          terms,
		ContractID:     allianceUUID,
		// Allies:         []tfcPb.Player{allies[0].Color, allies[1].Color},
	}

	collectionID := getCollectionID(allies[0].TFCClient, allies[1].TFCClient)
	alliTrxArgs := &tfcPb.AllianceTrxArgs{
		Type:         tfcPb.AllianceTrxType_INIT,
		InitPayload:  ad,
		CollectionID: collectionID,
		Allies:       []tfcPb.Player{allies[0].Color, allies[1].Color},
	}

	protoData, err := proto.Marshal(alliTrxArgs)
	if err != nil {
		return err
	}

	log.Printf("Installing the alliance chaincode...")

	_, err = invokeAndMeasure(allies[0].TFCClient, allianceName, "alliance", protoData)
	if err != nil {
		return err
	}

	registerAllianceListener(allies, allianceUUID, allianceName)

	return nil
}

func registerAllianceListener(allies []*ally, observerID uint32, allianceName string) *GameObserver {

	shutdown := make(chan bool, 100)
	trxComplete := make(chan *tfcPb.TrxCompletedArgs, 100)
	observer := &GameObserver{trxComplete, shutdown, allianceName, observerID, false}

	go handleAllianceEventsAsync(allies, observer)

	for _, a := range allies {
		a.GameObservers = append(a.GameObservers, observer)
	}

	return observer
}

func handleAllianceEventsAsync(allies []*ally, gameObserver *GameObserver) {
	defer recordFailure()
	for {
		select {
		case <-gameObserver.Shutdown:
			log.Println("received shutdown message...terminating")
			gameObserver.Terminate()
			return
		case ev := <-gameObserver.TrxComplete:
			log.Printf("received cc event...processing tx completed %v", ev)

			ev.ObserverID = gameObserver.ObserverID
			collectionID := getCollectionID(allies[0].TFCClient, allies[1].TFCClient)
			alliTrxArgs := &tfcPb.AllianceTrxArgs{
				Type:          tfcPb.AllianceTrxType_INVOKE,
				InvokePayload: ev,
				CollectionID:  collectionID,
				Allies:        []tfcPb.Player{allies[0].Color, allies[1].Color},
			}
			protoData, err := proto.Marshal(alliTrxArgs)
			if err != nil {
				panic(err)
			}

			r, err := invokeAndMeasure(allies[0].TFCClient, gameObserver.Name, "alliance", protoData)
			if err != nil {
				panic(err)
			}

			allianceResp := &tfcPb.AllianceData{}
			err = proto.Unmarshal(r.Payload, allianceResp)
			if err != nil {
				panic(fmt.Errorf("failed to unmarshal response %s, %v",
					r.Payload, err))
			}

			log.Printf("Got alliance response  %v", allianceResp)
			if allianceResp.State != tfcPb.AllianceState_ACTIVE {
				log.Println("Alliance completed, ending observer loop.")
				gameObserver.Terminate()
				return
			}

		default:
			log.Println("no messages received, sleeping a bit")
		}

		timeout, _ := time.ParseDuration("1s")
		time.Sleep(timeout)
	}

}

func invokeAndMeasure(player *TFCClient, ccName, ccLabel string, trxArgs []byte) (channel.Response, error) {

	st := time.Now()
	r, err := invokeGameChaincode(player, ccName, trxArgs)
	rt := time.Since(st).Seconds()

	if err != nil {
		player.Metrics.
			With(CCLabel, ccLabel).
			With(CCFailedLabel, "True").
			Observe(rt)
		return r, err
	}

	player.Metrics.
		With(CCLabel, ccLabel).
		With(CCFailedLabel, "False").
		Observe(rt)

	return r, nil
}

func invokeGameChaincode(player *TFCClient, ccName string, protoArgs []byte) (channel.Response, error) {

	log.Printf("Invoking game chaincode %s for client %v", ccName, player)

	response, err := player.ChannelClient.Execute(
		channel.Request{
			ChaincodeID: ccName,
			Fcn:         "publish",
			Args:        [][]byte{protoArgs}},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return channel.Response{}, fmt.Errorf("Failed to invoke cc: %s", err)
	}

	return response, nil
}

func recordFailure() {

	if r := recover(); r != nil {
		fmt.Println("Recovered from ops failure", r)
		GetPlayerMetrics().
			With(CCLabel, "Operations").
			With(CCFailedLabel, "True").
			Observe(1)
	}

}
