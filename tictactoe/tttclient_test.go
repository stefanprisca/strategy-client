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

	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/test/integration"
	"github.com/hyperledger/fabric-sdk-go/test/metadata"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/stretchr/testify/assert"
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
	ccPath           = "github.com/example_cc"
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

	org1MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org1))
	if err != nil {
		t.Fatal(err, "failed to create org1MspClient")
	}

	org2MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(org2))
	if err != nil {
		t.Fatal(err, "failed to create org2MspClient")
	}

	loadOrgPeers(t, sdk.Context(fabsdk.WithUser(org1AdminUser), fabsdk.WithOrg(org1)))

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
