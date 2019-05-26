package tfc

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"

	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	tfcPb "github.com/stefanprisca/strategy-protobufs/tfc"
)

type GameObserver struct {
	TrxComplete chan *tfcPb.TrxCompletedArgs
	Shutdown    chan bool
	Name        string
	UUID        uint32
	terminated  bool
}

func (gObs *GameObserver) Terminate() {
	close(gObs.Shutdown)
	close(gObs.TrxComplete)
	gObs.terminated = true
}

// OrgContext provides SDK client context for a given org
type TFCClient struct {
	OrgID                string
	CtxProvider          context.ClientProvider
	SigningIdentity      msp.SigningIdentity
	ResMgmt              *resmgmt.Client
	PeerEndpoint         string
	AnchorPeerConfigFile string
	Endorser             string
	SDK                  *fabsdk.FabricSDK
	ChannelClient        *channel.Client
	GameObservers        []*GameObserver
	Metrics              *prometheus.Histogram
}

const (
	Player1         = "Player1"
	Player2         = "Player2"
	Player3         = "Player3"
	Player4         = "Player4"
	Player5         = "Player5"
	AdminUser       = "Admin"
	OrdererOrg      = "Orderer"
	User            = "User1"
	OrdererEndpoint = "orderer.tfc.com"
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

func NewTFCClient(fabCfgPath, clientCfgPath, org, gameName string) (*TFCClient, error) {

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

	observers := []*GameObserver{}

	tfcClient := &TFCClient{
		OrgID:                org,
		CtxProvider:          adminContext,
		SigningIdentity:      orgIdentity,
		ResMgmt:              orgResMgmt,
		PeerEndpoint:         "peer0." + strings.ToLower(org) + ".tfc.com",
		AnchorPeerConfigFile: path.Join(fabCfgPath, org+"anchors.tx"),
		Endorser:             org + "MSP.peer",
		SDK:                  sdk,
		ChannelClient:        nil,
		GameObservers:        observers,
		Metrics:              nil,
	}

	return tfcClient, nil
}
