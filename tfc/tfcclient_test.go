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
	"os"
	"os/exec"
	"path"
	"testing"
	"text/template"

	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"

	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt" 
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	// "github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	
	"github.com/hyperledger/fabric-sdk-go/test/integration"

	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"

)

const (
	org1             = "Player1"
	org1Endpoint 	= "peer0.player1.tfc.com"
	org2             = "Player2"
	adminUser 		= "Admin"
	ordererOrg   = "Orderer"
	user         = "User1"
	ordererEndpoint  = "orderer.tfc.com"
)

var (
	scfixturesPath = path.Join(os.Getenv("SCFIXTURES"), "tfc")
	gopath = os.Getenv("GOPATH")
)

type ccDescriptor struct {
	ccID string
	ccPath string
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

	configPath := "./tfc_config.yaml"
	fmt.Println(configPath)
	configOpt := config.FromFile(configPath)

	sdk, err := fabsdk.New(configOpt)
	if err != nil {
		t.Fatalf("Failed to create new SDK: %s", err)
	}
	defer sdk.Close()

	// Delete all private keys from the crypto suite store
	// and users from the user store at the end
	integration.CleanupUserData(t, sdk)
	defer integration.CleanupUserData(t, sdk)


	chanName := "tttfan"

	cfgPath, err := generateChannelArtifacts(chanName)
	require.NoError(t, err)

	chanTxPath := path.Join(cfgPath, chanName+".tx")
	err = createChannel(sdk, chanName, chanTxPath, org1)
	require.NoError(t, err)

	err = joinChannel(sdk, chanName, cfgPath, org1)
	require.NoError(t, err)

	ccPath := "github.com/stefanprisca/strategy-code/tictactoe"
	ccPkg, err := createCC(ccPath)
	require.NoError(t, err)

	ccDesc := ccDescriptor{
		ccID: chanName,
		ccPath: ccPath,
		ccVersion: "0.1.0",
		ccPackage: ccPkg,
	}

	err = installChaincode(sdk, ccDesc, org1, chanName)
	require.NoError(t, err)

}

func createChannel(sdk *fabsdk.FabricSDK, chanName, cfgPath, org string) error {

	client, err := mspclient.New(sdk.Context(), mspclient.WithOrg(org))
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}
	orgIdentity, err := client.GetSigningIdentity(adminUser)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	adminContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(ordererOrg))
	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	r, err := os.Open(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to open channel config: %s", err)
	}
	defer r.Close()

	resp, err := orgResMgmt.SaveChannel(
		resmgmt.SaveChannelRequest{
			ChannelID: chanName, 
			ChannelConfig: r, 
			SigningIdentities: []msp.SigningIdentity{orgIdentity}, 
			},
		resmgmt.WithOrdererEndpoint(ordererEndpoint),
		resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return fmt.Errorf("failed to save channel: %s", err)
	}

	if resp.TransactionID == "" {
		return fmt.Errorf("Failed to save channel")
	}

	// req := resmgmt.SaveChannelRequest{ChannelID: chanName,
	// 	ChannelConfigPath: cfgPath,
	// 	SigningIdentities: []msp.SigningIdentity{orgIdentity}}

	// _, err = orgResMgmt.SaveChannel(req,
	// 	resmgmt.WithRetry(retry.DefaultResMgmtOpts),
	// 	resmgmt.WithOrdererEndpoint(ordererEndpoint))

	// if err != nil {
	// 	return fmt.Errorf("could not create channel: %s", err)
	// }
	return nil
}

func joinChannel(sdk *fabsdk.FabricSDK, chanName, cfgPath, org string) error {

	adminContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(org))

	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	client, err := mspclient.New(sdk.Context(), mspclient.WithOrg(org))
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}
	orgIdentity, err := client.GetSigningIdentity(adminUser)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	req := resmgmt.SaveChannelRequest{
		ChannelID:         chanName,
		ChannelConfigPath: path.Join(cfgPath, org+"anchors.tx"),
		SigningIdentities: []msp.SigningIdentity{orgIdentity},
	}
	if _, err := orgResMgmt.SaveChannel(req,
		 resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		 resmgmt.WithOrdererEndpoint(ordererEndpoint)); err != nil {
		return err
	}


	// Org peers join channel
	if err := orgResMgmt.JoinChannel(chanName, resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(ordererEndpoint),
		resmgmt.WithTargetEndpoints(org1Endpoint)); err != nil {
		return fmt.Errorf("Org peers failed to JoinChannel: %s", err)
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

func installChaincode(sdk *fabsdk.FabricSDK, ccDesc ccDescriptor, org, chanName string) error {

	adminContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(org))

	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	// Install example cc to org peers
	installCCReq := resmgmt.InstallCCRequest{Name: ccDesc.ccID, Path: ccDesc.ccPath, Version: ccDesc.ccVersion, Package: ccDesc.ccPackage}
	_, err = orgResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return fmt.Errorf("failed to install cc: %s",err)
	}
	// Set up chaincode policy
	ccPolicy := cauthdsl.AcceptAllPolicy
	// Org resource manager will instantiate 'example_cc' on channel
	_, err = orgResMgmt.InstantiateCC(
		chanName,
		resmgmt.InstantiateCCRequest{
			Name: ccDesc.ccID, 
			Path: ccDesc.ccPath, 
			Version: ccDesc.ccVersion, 
			Args: integration.ExampleCCInitArgs(),
			Policy:     ccPolicy,
		},
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
	)
	return err
}

type chanTemplateData struct {
	Red   string
	Green string
	Blue  string
}

func TestChanGen(t *testing.T) {
	_, err := generateChannelArtifacts("foo")
	require.NoError(t, err)
}

func generateChannelArtifacts(channelName string) (string, error) {
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

	chanTemplate.Execute(cfgFile, chanTemplateData{"Player1", "Player2", "Player3"})

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


