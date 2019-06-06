curDir=$(pwd)
echo "Saving current directory $curDir"

cd ~/workspace/hyperledger/strategy-chains/tfc/
yes | ./tfc.sh upCC
cd $curDir

go test -run TestE2ETTT > reports/logs/ttt_$(date -Iminutes)

cd ~/workspace/hyperledger/strategy-chains/tfc/
yes | ./tfc.sh down
cd $curDir

go test -run TestE2ETFC > reports/logs/tfc_$(date -Iminutes)

cd ~/workspace/hyperledger/strategy-chains/tfc/
yes | ./tfc.sh down
cd $curDir

go test -run TestGoroutinesIncremental > reports/logs/inc_$(date -Iminutes)

cd ~/workspace/hyperledger/strategy-chains/tfc/
yes | ./tfc.sh down
cd $curDir
