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
	"testing"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/test/integration"
	"github.com/hyperledger/fabric-sdk-go/test/metadata"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ccId             = "ttt"
)

var (
	// SDK
	sdk *fabsdk.FabricSDK

	// Org MSP clients
	org1MspClient *mspclient.Client
	org2MspClient *mspclient.Client
	// Peers
	orgTestPeer0 fab.Peer
	orgTestPeer1 fab.Peer
	exampleCC    = "example_cc_e2e" + metadata.TestRunID
)

type multiorgContext struct {
	// client contexts
	ordererClientContext   contextAPI.ClientProvider
	org1AdminClientContext contextAPI.ClientProvider
	org2AdminClientContext contextAPI.ClientProvider
	org1ResMgmt            *resmgmt.Client
	org2ResMgmt            *resmgmt.Client
	ccName                 string
	ccVersion              string
	channelID              string
}

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

	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser(org1User), fabsdk.WithOrg(org1))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	require.NoError(t, err, "could not get channel client")

	response, err := client.Query(channel.Request{ChaincodeID: ccID, Fcn: "invoke", Args: integration.ExampleCCDefaultQueryArgs()},
		channel.WithRetry(retry.DefaultChannelOpts),
		channel.WithTargetEndpoints(targetEndpoints...),
	)
	if err != nil {
		t.Fatalf("Failed to query funds: %s", err)
	}

	return

	// org1MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org1))
	// if err != nil {
	// 	t.Fatal(err, "failed to create org1MspClient")
	// }

	// org2MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org2))
	// if err != nil {
	// 	t.Fatal(err, "failed to create org2MspClient")
	// }

	// loadOrgPeers(t, sdk.Context(fabsdk.WithUser(org1AdminUser), fabsdk.WithOrg(org1)))
	// loadOrgPeers(t, sdk.Context(fabsdk.WithUser(org2AdminUser), fabsdk.WithOrg(org2)))

	// mc := multiorgContext{
	// 	ordererClientContext:   sdk.Context(fabsdk.WithUser(ordererAdminUser), fabsdk.WithOrg(ordererOrgName)),
	// 	org1AdminClientContext: sdk.Context(fabsdk.WithUser(org1AdminUser), fabsdk.WithOrg(org1)),
	// 	org2AdminClientContext: sdk.Context(fabsdk.WithUser(org2AdminUser), fabsdk.WithOrg(org2)),
	// 	ccName:                 exampleCC, // basic multi orgs test uses exampleCC for testing
	// 	ccVersion:              "0",
	// 	channelID:              channelID,
	// }

	// org1Peers, err := integration.DiscoverLocalPeers(mc.org1AdminClientContext, 1)
	// require.NoError(t, err)
	// _, err = integration.DiscoverLocalPeers(mc.org2AdminClientContext, 1)
	// require.NoError(t, err)

	// org1RMgmt, err := resmgmt.New(mc.org1AdminClientContext)
	// require.NoError(t, err, "failed to create org1 resource management client")

	// mc.org1ResMgmt = org1RMgmt

	// // Org2 resource management client
	// org2RMgmt, err := resmgmt.New(mc.org2AdminClientContext)
	// require.NoError(t, err, "failed to create org2 resource management client")

	// mc.org2ResMgmt = org2RMgmt
	// joined, err := integration.IsJoinedChannel(channelID, mc.org1ResMgmt, org1Peers[0])
	// require.NoError(t, err)
	// fmt.Println("If it joined I go smoke.", joined)
}

func loadOrgPeers(t *testing.T, ctxProvider contextAPI.ClientProvider) {

	ctx, err := ctxProvider()
	if err != nil {
		t.Fatalf("context creation failed: %s", err)
	}

	org1Peers, ok := ctx.EndpointConfig().PeersConfig(org1)
	assert.True(t, ok)

	org2Peers, ok := ctx.EndpointConfig().PeersConfig(org2)
	assert.True(t, ok)

	_, err = ctx.InfraProvider().CreatePeerFromConfig(&fab.NetworkPeer{PeerConfig: org1Peers[0]})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ctx.InfraProvider().CreatePeerFromConfig(&fab.NetworkPeer{PeerConfig: org2Peers[0]})
	if err != nil {
		t.Fatal(err)
	}

}
