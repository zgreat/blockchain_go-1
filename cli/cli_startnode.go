package main

import (
	"fmt"
	"log"
	"../blockchain_go"
	"../p2pprotocol"
)

func (cli *CLI) startNode(nodeID, minerAddress string) {
	fmt.Printf("Starting node %s\n", nodeID)
	if len(minerAddress) > 0 {
		if core.ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}

	p2pprotocol.StartServer(nodeID, minerAddress)
}
