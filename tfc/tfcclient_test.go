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
	ordererAdminUser = "Admin"
	ordererOrgName   = "OrdererOrg"
	org1AdminUser    = "Admin"
	org2AdminUser    = "Admin"
	org1User         = "User1"
	org2User         = "User1"
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

	adminContext := sdk.Context(fabsdk.WithUser(org1AdminUser), fabsdk.WithOrg(org1))

	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		t.Fatalf("Failed to create new resource management client: %s", err)
	}

	mspClient, err := mspclient.New(sdk.Context(), mspclient.WithOrg(org1))
	if err != nil {
		t.Fatal(err)
	}
	adminIdentity, err := mspClient.GetSigningIdentity(org1AdminUser)
	if err != nil {
		t.Fatal(err)
	}

	chanName := "testchan"

	cfgPath, err := generateChannelArtifacts(chanName)
	require.NoError(t, err)

	req := resmgmt.SaveChannelRequest{ChannelID: chanName,
		ChannelConfigPath: cfgPath,
		SigningIdentities: []msp.SigningIdentity{adminIdentity}}

	txID, err := orgResMgmt.SaveChannel(req,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(ordererEndpoint))

	require.Nil(t, err, "error should be nil")
	require.NotEmpty(t, txID, "transaction ID should be populated")

	// Org peers join channel
	if err = orgResMgmt.JoinChannel(chanName, resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint("orderer.tfc.com")); err != nil {
		t.Fatalf("Org peers failed to JoinChannel: %s", err)
	}

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
