# Getting Started

This provides a step-by-step guide to install all components required to run the TFC and TTT systems and experiments locally. The guide targets the TFC network which is more complex, and by default contains both the TTT and TFC. There is a simpler network specifically for the TTT game, but this network is obsolete and has not been maintained. Before moving on, it is important to note that this guide is specific for Linux distributions, and the steps presented bellow are not guaranteed to work on other platforms.

# Prerequisites

First and foremost, the Hyperledger Fabric binaries and dependencies need to be installed. To do so, follow the Hyperledger Getting
Started tutorial available here: <https://hyperledger-fabric.readthedocs.io/en/latest/getting_started.html>. At the end of this guide, you
should have the following components:
* Google Golang programming environment.
* Local docker client. It is required to run the Hyperledger images.
* Hyperledger Fabric samples folder. In the guide, you are required to clone the Fabric samples repository and install all the binaries under the /bin directory.

In order to boostrap the blockchain network for the two systems, the Hyperledger binaries are required. To make it easier to access them, it is  recommended that you add the path of the Fabric /bin folder to the system PATH variable. Assuming that the Fabric samples were installed in the fabric-samples folder, adding this to the system path can be done with the following command:

`export PATH = $PATH : path/to/fabric − samples/bin`

The next part is to create an `SCFIXTURES`  folder where the network artifacts can be saved during generation. It does not matter where this
folder is created, as long as it is accessible by the network setup script. The following commands create the root `SCFIXTURES` folder under
the `/home/$USER/.strategy-chains/`, the necessary tfc fixtures folder, and export the `$SCFIXTURES` system variable:

```
mkdir /home/$USER/.strategy-chains/
mkdir /home/$USER/.strategy-chains/tfc
export SCFIXTURES=/home/$USER/.strategy-chains/
```

The last prerequisite to running the system is Prometheus. It is required by the experiments to export the observations. Follow this tutorial to set it up: https://prometheus.io/docs/introduction/first_steps/. The configuration required for scraping the Prometheus server running inside the experiments and the cadvisor docker monitoring container is given bellow:

```
scrape_configs:
 - job_name: ’demo’
    # scrape the service every second 
    scrape_interval: 1s
    # setup the static configs
    static_configs:
        - targets: [’127.0.0.1:9009’]
        - job_name: cadvisor
    scrape_interval: 5s
    static_configs:
    - targets:
     - 127.0.0.1:8080
```

# Obtaining the sources

To get started with the project, you need the source code. This is split between the following repositories:
* strategy-network https://github.com/stefanprisca/strategy-network : contains the network definition, together with the tools required to start it.
* strategy-code https://github.com/stefanprisca/strategy-code : contains the chaincode implementation, and tests for it.
* strategy-client https://github.com/stefanprisca/strategy-client contains the client application.
* strategy-protobufs https://github.com/stefanprisca/strategy-protobufs contains the protobuf definitions for smart contracts.

The four repositories need to be cloned in the local working environment. There is no folder structure enforced, as all the components are independent from each other. Moreover, it is not necessary to clone them under the GOPATH system variable, as the new go modules are used to manage the dependencies between them.

# Starting the TFC network

Having all the prerequisites in place, let’s see how to get the network up and running. This can be done by using the following commands, executed from the strategy-workspace folder:
```
cd strategy-network/tfc
./tfc.sh upCC
```

The `tfc.sh` asks in the beginning if you want to proceed creating the network. Answering yes will create and start the docker containers and install the chaincode for tic tac toe and TFC. If the script succeeds, the output is similar to the one in Figure 35. Some of the other options to the tfc.sh command are:
* `./tfc.sh up` command brings up the network without install the chaincodes.
* `./tfc.sh down` command shuts down the network and cleans the `SCFIXTURES` folder by removing all artifacts which were generated as part of this session.
* `./tfc.sh test` brings up the network and runs a test script to verify the connectivity of the network components.

## Starting the TTT Network

Using the TTT network is not recommended, as it has not been maintained and updated to match the functions of the TFC network. However, it can be started in the same way as the TFC network.
The folder containing the configuration files is located under `strategynetwork/ttt`. Starting the network can be done by running the provided
script: `./ttt.sh up`. This script creates a channel by default, and installs and instantiates the TTT chaincode on it.

# Running Experiments

After the network is running, the client application experiments from `strategy-client/perfTest` can be executed to test the system. There are
three experiments currently available in the tfcclient_test.go file:
* TestE2ETTT runs an instance of tic tac toe on the network.
* TestE2ETFC runs an instance of TFC on the network.
* TestGoroutinesIncremental runs both TTT and TFC games in an incremental manner.

All of these tests start a Prometheus server which can be scrapped for metrics by the local Prometheus service. Make sure to have the service up an running, and start it using the configuration given in this tutorial.

The TestE2ETTT experiment can be ran directly, without any extra setup, using the following commands from the strategy-workspace
folder:
```
cd strategy-client/perfTest
go test -run TestE2ETTT
```

The produced output will contain the test logs for the experiment.

For the TestE2ETFC experiment, there are some setup steps before it can be executed. These are required as alliances are installed from the client application. To do this, the alliance chaincode needs to be available under the local GOPATH, as the client application looks for it there. In order to setup the local alliance chaincode, use the following commands from the strategy-workspace folder:

```
mkdir -p $GOPATH/src/local-cc/alliance/
cp strategy-code/cmd/alliance/ $GOPATH/src/local-cc/ -rdf
cd $GOPATH/src/local-cc/alliance
go mod init
go build
go mod vendor
```
The first three commands will create the local alliance chaincode folder. Do not modify this path, as the client application expects the alliance chaincode to be under local-cc/alliance. The next three commands create a go module in the current folder, build the alliance chaincode and create a vendor folder for its dependencies. The vendor folder is required by Hyperledger to run the chaincode inside the network. After these setup steps, the TestE2ETFC can be executed similarly to TestE2ETTT:
```
cd strategy-client/perfTest
go test -run TestE2ETFC
```

Finally, the `TestGoroutinesIncremental` experiment can be ran in the same way as the TestE2ETFC. It also requires the alliance chaincode to be present under the local GOPATH, but the setup steps for this don’t need to be executed again. It suffices to create the local-cc/alliance folder once and all experiments can be executed after. In order to verify the output produced by Prometheus, visit the `http://localhost:9090/graph` web page. The metric used by the experiments to record observations is tfc_testing_runtime.

# Contributing to the Projects

All contributions and improvements are welcomed. However, there is no continuous development process setup for these projects. Contributions can be done through pull request to the github repositories, which will then be manually validated and merged. 

For both client application and the network, the changes can be tested locally. However, as chaincodes are installed on the network from their github repositories, validating the changes is not that easy.
There are unit tests for the strategy-code project to ensure that nothing breaks, but in order to see the changes in the real network you either have to push and update the master branch of the repository, or change the network setup script to install the local chaincode (the scripts/ccman.sh file).