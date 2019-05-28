package tfc

import (
	"fmt"
	"log"
	"strings"

	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
)

func getCollectionDefinitions(players []*TFCClient) ([]*common.CollectionConfig, error) {

	collections := []*common.CollectionConfig{}

	nOfPlayers := len(players)
	for i := range players {
		for j := i + 1; j < nOfPlayers; j++ {
			p1 := players[i]
			p2 := players[j]
			c, err := genStaticColConfig(p1, p2)
			if err != nil {
				return nil, err
			}
			collections = append(collections, c)
		}
	}

	log.Printf("Generated collection configurations %v", collections)

	return collections, nil
}

func genStaticColConfig(p1, p2 *TFCClient) (*common.CollectionConfig, error) {

	colPolicyStr := fmt.Sprintf("OR('%sMSP.member', '%sMSP.member')", p1.OrgID, p2.OrgID)
	log.Printf("Created policy string: %s", colPolicyStr)
	colPolicy, err := cauthdsl.FromString(colPolicyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create ccPolicy: %s", err)
	}

	colName := getCollectionID(p1, p2)

	stCol := &common.StaticCollectionConfig{
		Name:              colName,
		BlockToLive:       0,
		MaximumPeerCount:  2,
		RequiredPeerCount: 0,
		MemberOnlyRead:    true,
		MemberOrgsPolicy: &common.CollectionPolicyConfig{
			Payload: &common.CollectionPolicyConfig_SignaturePolicy{
				SignaturePolicy: colPolicy,
			},
		},
	}

	colConfig := &common.CollectionConfig{
		Payload: &common.CollectionConfig_StaticCollectionConfig{stCol},
	}

	return colConfig, nil
}

func getCollectionID(p1, p2 *TFCClient) string {
	return fmt.Sprintf("al%v%v", strings.ToLower(p1.OrgID), strings.ToLower(p2.OrgID))
}
