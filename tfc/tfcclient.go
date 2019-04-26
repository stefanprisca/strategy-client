package tfc

import (
	"fmt"
	"strings"
	"path"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"

	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
)

// OrgContext provides SDK client context for a given org
type TFCClient struct {
	OrgID           	 string
	CtxProvider          context.ClientProvider
	SigningIdentity      msp.SigningIdentity
	ResMgmt              *resmgmt.Client
	PeerEndpoint         string
	AnchorPeerConfigFile string
	SDK                  *fabsdk.FabricSDK
}


const (
	Org1             = "Player1"
	Org2             = "Player2"
	Org3             = "Player3"
	Org1Endpoint 	= "peer0.player1.tfc.com"
	AdminUser 		= "Admin"
	OrdererOrg   = "Orderer"
	User         = "User1"
	OrdererEndpoint  = "orderer.tfc.com"
)


func NewTFCClient(fabCfgPath, clientCfgPath, org string) (*TFCClient, error) {

	configOpt := config.FromFile(clientCfgPath)
	sdk, err := fabsdk.New(configOpt)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new SDK: %s", err)
	}

	adminContext := sdk.Context(fabsdk.WithUser(AdminUser), fabsdk.WithOrg(org))
	// Org resource management client
	orgResMgmt, err := resmgmt.New(adminContext)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new resource management client: %s", err)
	}
	client, err := mspclient.New(sdk.Context(), mspclient.WithOrg(org))
	if err != nil {
		return nil, fmt.Errorf("Failed to create new resource management client: %s", err)
	}
	orgIdentity, err := client.GetSigningIdentity(AdminUser)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new resource management client: %s", err)
	}

	tfcClient := &TFCClient{
		OrgID: org,
		CtxProvider: adminContext,
		SigningIdentity: orgIdentity,
		ResMgmt : orgResMgmt,
		PeerEndpoint: "peer0."+strings.ToLower(org)+".tfc.com",
		AnchorPeerConfigFile: path.Join(fabCfgPath, org+"anchors.tx"),
		SDK: sdk,
	}
	
	return tfcClient, nil
}
