function generateChannelArtifacts() {
  which configtxgen
  if [ "$?" -ne 0 ]; then
    echo "configtxgen tool not found. exiting"
    exit 1
  fi

  echo "##########################################################"
  echo "#########  Generating Orderer Genesis block ##############"
  echo "##########################################################"
  # Note: For some unknown reason (at least for now) the block file can't be
  # named orderer.genesis.block or the orderer will fail to launch!
  set -x
  configtxgen -profile TFCDevModeKafka -channelID tfc-sys-channel -outputBlock ${FABRIC_CFG_PATH}/channel-artifacts/genesis.block
  res=$?
  set +x
  if [ $res -ne 0 ]; then
    echo "Failed to generate orderer genesis block..."
    exit 1
  fi
  echo
  echo "#################################################################"
  echo "### Generating channel configuration transaction 'channel.tx' ###"
  echo "#################################################################"
  set -x
  configtxgen -profile TFCChannel -outputCreateChannelTx ${FABRIC_CFG_PATH}/channel-artifacts/channel.tx -channelID $CHANNEL_NAME
  res=$?
  set +x
  if [ $res -ne 0 ]; then
    echo "Failed to generate channel configuration transaction..."
    exit 1
  fi

#   echo
#   echo "#################################################################"
#   echo "#######    Generating anchor peer update for Player1  ##########"
#   echo "#################################################################"
#   set -x
#   configtxgen -profile TFCChannel -outputAnchorPeersUpdate ${FABRIC_CFG_PATH}/channel-artifacts/Player1anchors.tx -channelID $CHANNEL_NAME -asOrg Player1
#   res=$?
#   set +x
#   if [ $res -ne 0 ]; then
#     echo "Failed to generate anchor peer update for Player1..."
#     exit 1
#   fi

#   echo
#   echo "#################################################################"
#   echo "#######    Generating anchor peer update for Player2   ##########"
#   echo "#################################################################"
#   set -x
#   configtxgen -profile TFCChannel -outputAnchorPeersUpdate \
#     ${FABRIC_CFG_PATH}/channel-artifacts/Player2anchors.tx -channelID $CHANNEL_NAME -asOrg Player2
#   res=$?
#   set +x
#   if [ $res -ne 0 ]; then
#     echo "Failed to generate anchor peer update for Player2MSP..."
#     exit 1
#   fi
#   echo


#   echo
#   echo "#################################################################"
#   echo "#######    Generating anchor peer update for Player3   ##########"
#   echo "#################################################################"
#   set -x
#   configtxgen -profile TFCChannel -outputAnchorPeersUpdate \
#     ${FABRIC_CFG_PATH}/channel-artifacts/Player3anchors.tx -channelID $CHANNEL_NAME -asOrg Player3
#   res=$?
#   set +x
#   if [ $res -ne 0 ]; then
#     echo "Failed to generate anchor peer update for Player3MSP..."
#     exit 1
#   fi
#   echo

#   echo
#   echo "#################################################################"
#   echo "#######    Generating anchor peer update for Player4   ##########"
#   echo "#################################################################"
#   set -x
#   configtxgen -profile TFCChannel -outputAnchorPeersUpdate \
#     ${FABRIC_CFG_PATH}/channel-artifacts/Player4anchors.tx -channelID $CHANNEL_NAME -asOrg Player4
#   res=$?
#   set +x
#   if [ $res -ne 0 ]; then
#     echo "Failed to generate anchor peer update for Player4MSP..."
#     exit 1
#   fi
#   echo

#   echo
#   echo "#################################################################"
#   echo "#######    Generating anchor peer update for Player5   ##########"
#   echo "#################################################################"
#   set -x
#   configtxgen -profile TFCChannel -outputAnchorPeersUpdate \
#     ${FABRIC_CFG_PATH}/channel-artifacts/Player5anchors.tx -channelID $CHANNEL_NAME -asOrg Player5
#   res=$?
#   set +x
#   if [ $res -ne 0 ]; then
#     echo "Failed to generate anchor peer update for Player5MSP..."
#     exit 1
#   fi
#   echo
}

CHANNEL_NAME=$1
generateChannelArtifacts