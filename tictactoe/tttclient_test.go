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
package tictactoe

import (
	"fmt"
	"log"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/test/integration"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/stretchr/testify/require"

	tttPf "github.com/stefanprisca/strategy-protobufs/tictactoe"
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
	channelID        = "tttchannel"
)

/*
	Test that will
	1) Create the sdk
	2) Connect both players
	3) Play a game
*/
func TestE2E(t *testing.T) {

	configPath := "./ttt_config.yaml"
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

	ccResp, err := orgResMgmt.QueryInstantiatedChaincodes(channelID)
	require.NoError(t, err, "Could not get ccs")
	fmt.Println("Got the chaincodes installed", ccResp.Chaincodes)

	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser(org1User), fabsdk.WithOrg(org1))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	require.NoError(t, err, "could not get channel client")
	log.Println("Connected client for player1")

	mvPayload := &tttPf.MoveTrxPayload{Mark: tttPf.Mark_X, Position: 3}
	trxArgs := &tttPf.TrxArgs{Type: tttPf.TrxType_MOVE, MovePayload: mvPayload}

	trxBytes, err := proto.Marshal(trxArgs)
	require.NoError(t, err, "Could not marshal trx args.")
	fmt.Println(trxBytes)

	ccName := ccResp.Chaincodes[0].GetName()
	response, err := client.Execute(channel.Request{ChaincodeID: ccName, Fcn: "move", Args: [][]byte{trxBytes}},
		channel.WithRetry(retry.DefaultChannelOpts))
	if err != nil {
		t.Fatalf("Failed to invoke cc: %s", err)
	}
	log.Println("Issued chaincode invoke.")
	gBoardBytes := response.Payload
	gBoard := &tttPf.TttContract{}
	err = proto.Unmarshal(gBoardBytes, gBoard)
	require.NoError(t, err, "Could not unmarshal response from Tictactoe!")

	fmt.Println(gBoard.GetPositions())
}
