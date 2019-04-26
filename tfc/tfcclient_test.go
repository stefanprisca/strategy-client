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

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/test/integration"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"

)

const (
	org1             = "Player1"
	org2             = "Player2"
	adminUser 		= "Admin"
	ordererOrg   = "Orderer"
	user         = "User1"
	ordererEndpoint  = "orderer.tfc.com"
)

var (
	scfixturesPath = path.Join(os.Getenv("SCFIXTURES"), "tfc")
)

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


	chanName := "testchan2"

	// cfgPath, err := generateChannelArtifacts(chanName)
	// require.NoError(t, err)

	// err = createChannel(sdk, chanName, cfgPath, org1)
	// require.NoError(t, err)

	err = joinChannel(sdk, chanName, org1)
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

	req := resmgmt.SaveChannelRequest{ChannelID: chanName,
		ChannelConfigPath: cfgPath,
		SigningIdentities: []msp.SigningIdentity{orgIdentity}}

	_, err = orgResMgmt.SaveChannel(req,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(ordererEndpoint))

	if err != nil {
		return fmt.Errorf("could not create channel: %s", err)
	}
	return nil
}

func joinChannel(sdk *fabsdk.FabricSDK, chanName, org string) error {

	adminContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(org))

	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	// Org peers join channel
	if err := orgResMgmt.JoinChannel(chanName, resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(ordererEndpoint)); err != nil {
		return fmt.Errorf("Org peers failed to JoinChannel: %s", err)
	}
	return nil
}

type chanTemplateData struct {
	Red   string
	Green string
	Blue  string
}

func _TestChanGen(t *testing.T) {
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
	fmt.Println(cfgPath)
	channelTxPath := path.Join(cfgPath, channelName+".tx")

	configtxgen := exec.Command("configtxgen",
		"-profile", "TFCChannel",
		"-outputCreateChannelTx", channelTxPath,
		"-channelID", channelName)

	result, err := configtxgen.CombinedOutput()
	print(result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute commad. %s \n %s", err.Error(), string(result))
	}

	return channelTxPath, nil
}


