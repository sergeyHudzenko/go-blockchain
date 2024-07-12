package main

import (
	"fmt"

	bc "github.com/sergeyHudzenko/go-blockchain/blockchain"
)

const (
	DBNAME = "blockchain.db"
)

func main() {
	miner := bc.NewUser()
	bc.NewChain(DBNAME, miner.Address())
	chain := bc.LoadChain(DBNAME)
	if chain == nil {
		return
	}

	for i:=0; i<3; i++ {
		block := bc.NewBlock(miner.Address(), chain.LastHash())
		block.AddTransaction(chain, bc.NewTransaction(miner, chain.LastHash(), "AAA", 5))
		block.AddTransaction(chain, bc.NewTransaction(miner, chain.LastHash(), "BBB", 10))
		block.Accept(chain, miner, make(chan bool))
		chain.AddBlock(block)
	}
	// PRINT CHAIN
	var sblock string
	rows, err := chain.DB.Query("SELECT Block FROM BlockChain")
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		rows.Scan(&sblock)
		fmt.Println(sblock)
	}
}