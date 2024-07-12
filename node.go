package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	bc "github.com/sergeyHudzenko/go-blockchain/blockchain"
	nt "github.com/sergeyHudzenko/go-blockchain/network"
)

var (
	Addresses []string
	User *bc.User
	Serve string
	Chain *bc.BlockChain
	Block *bc.Block
	Filename string
	Mutex sync.Mutex
	IsMining bool
	BreakMining  = make(chan bool)
)

const (
	SEPARATOR = "_SEPARATOR_"
)

const (
	ADD_BLOCK = iota + 1
	ADD_TRNSX
	GET_BLOCK
	GET_LHASH
	GET_BLNCE
)


func init() {
	if len(os.Args) < 2 {
		panic("Failed #1")
	}

	var (
		addrStr = ""
		userNewStr = ""
		userLoadStr=""
		serveStr = ""
		chainLoadStr = ""
		chainNewStr = ""
	)
 
	var (
		addrExist = false
		userNewExist = false
		userLoadExist = false
		serveExist = false
		chainNewExist = false
		chainLoadExist = false
	)
	
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case strings.HasPrefix(arg, "-serve:"):
			serveStr = strings.Replace(arg, "-serve:", "",1)
			serveExist = true
		case strings.HasPrefix(arg, "-loadchain:"):
			chainLoadStr = strings.Replace(arg, "-loadchain:", "",1)
			chainLoadExist = true
		case strings.HasPrefix(arg, "-newchain:"):
			chainNewStr = strings.Replace(arg, "-newchain:", "",1)
			chainNewExist = true
		case strings.HasPrefix(arg, "-loadaddr:"):
			addrStr = strings.Replace(arg, "-loadaddr:", "",1)
			addrExist = true
		case strings.HasPrefix(arg, "-newuser:"):
			userNewStr = strings.Replace(arg, "-newuser:", "",1)
			userNewExist = true
		case strings.HasPrefix(arg, "-loaduser:"):
			userLoadStr = strings.Replace(arg, "-loaduser:", "",1)
			userLoadExist = true
		}
	}
	
	if !addrExist || !(userLoadExist || userNewExist) || !serveExist || !(chainLoadExist || chainNewExist) {
		panic("Failed 2")
	}

	Serve = serveStr
	var addresses []string

	err := json.Unmarshal([]byte(readFile(addrStr)), &addresses)
	if err != nil {
		panic("Failed 3")
	}

	var mapaddr = make(map[string]bool)

	for _, addr := range addresses {
		if addr == Serve {
			continue
		}

		if _, ok := mapaddr[addr]; ok {
			continue
		}

		mapaddr[addr] = true
		Addresses = append(Addresses, addr)
	}

	if userNewExist {
		User = userNew(userNewStr)
	}

	if userLoadExist {
		User = userLoad(userLoadStr)
	}

	if User == nil {
		panic("Failed 5")
	}

	if chainNewExist {
		Filename = chainNewStr
		Chain = chainNew(chainNewStr)
	}

	if chainLoadExist {
		Filename = chainLoadStr
		Chain = chainLoad(chainLoadStr)
	}

	if Chain == nil {
		panic("Failed 6")
	}

	Block = bc.NewBlock(User.Address(), Chain.LastHash())
}

func readFile(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(data)
}

func writeFile(filename, data string) error {
	return os.WriteFile(filename, []byte(data), 0644) 
}

func userNew(filename string) *bc.User {
	user := bc.NewUser()
	if user == nil {
		return nil
	}
	err := writeFile(filename, user.Purse())
	if err != nil {
		return nil
	}

	return user
}

func userLoad(filename string) *bc.User {
	priv := readFile(filename)
		if priv == "" {
			return nil
		}
		user := bc.LoadUser(priv)
		if user == nil {
			return nil
		}

		return user
}

func chainNew(filename string) *bc.BlockChain {
	bc.NewChain(filename, User.Address())
	return bc.LoadChain(filename)
}

func chainLoad(filename string) *bc.BlockChain {
	chain := bc.LoadChain(filename)
	return chain
}

func handleServer(conn nt.Conn, pack *nt.Package) {
	nt.Handle(ADD_BLOCK, conn, pack, addBlock)
	nt.Handle(ADD_TRNSX, conn, pack, addTransaction)
	nt.Handle(GET_BLOCK, conn, pack, getBlock)
	nt.Handle(GET_LHASH, conn, pack, getLastHash)
	nt.Handle(GET_BLNCE, conn, pack, getBalance)
}

func main() {
	nt.Listen(Serve, handleServer)
	for {
		fmt.Scanln()
	}
}

func addBlock(pack *nt.Package) string {
	splited := strings.Split(pack.Data, SEPARATOR)
	if len(splited) != 3 {
		return "fail"
	}
	block := bc.DeserializeBlock(splited[2])
	if !block.IsValid(Chain) {
		currSize := Chain.Size()
		num, err := strconv.Atoi(splited[1])
		if err != nil {
			return "fail"
		}

		if currSize < uint64(num) {
			go compareChains(splited[0], uint64(num))
			return "ok"
		}

		return "fail"
	}
	
	Mutex.Lock()
	Chain.AddBlock(block)
	Block = bc.NewBlock(User.Address(), Chain.LastHash())
	Mutex.Unlock()

	if IsMining {
		BreakMining <- true
		IsMining = false
	}

	return "ok"
}


func addTransaction(pack *nt.Package) string {
	var tx = bc.DeserializeTX(pack.Data)
	if tx == nil || len (Block.Transactions) == bc.TX_LIMIT { 
		return "fail"
	}

	Mutex.Lock()
	err := Block.AddTransaction(Chain, tx)
	Mutex.Unlock()

	if err != nil {
		return "fail"
	}
	// If the last transaction start creating new block
	if len(Block.Transactions) == bc.TX_LIMIT {
		go func() {
			Mutex.Lock()
			block := *Block
			IsMining = true
			Mutex.Unlock()
			err := (&block).Accept(Chain, User, BreakMining)
			Mutex.Lock()
			IsMining = false
			if err == nil && bytes.Equal(block.PrevHash, Block.PrevHash) {
				Chain.AddBlock(&block) 
				pushBlockToNet(&block)
			}
			Block = bc.NewBlock(User.Address(), Chain.LastHash())
			Mutex.Unlock()
		}()
	}

	return "ok"
}

func getBlock(pack *nt.Package) string {
	num, err := strconv.Atoi(pack.Data)
	if err != nil {
		return ""
	}
	size := Chain.Size()
	if uint64 (num) < size {
		return selectBlock(Chain, num)
	}
	return ""
}

func getLastHash (pack *nt.Package) string {
	return bc.Base64Encode(Chain.LastHash())
}

func getBalance(pack *nt.Package) string {
	return fmt.Sprintf("%d", Chain.Balance(pack.Data))
}

func compareChains(address string, num uint64) {
	filename := "temp_" + hex.EncodeToString(bc.GenerateRandomBytes(8))
	file, err := os.Create(filename)
	if err != nil {
		return
	}
	file.Close()
	defer func() {
		os.Remove(filename)
	}()

	res := nt.Send(address, &nt.Package {
		Option: GET_BLOCK,
		Data: fmt.Sprintf("%d", 0),
	})

	if res == nil {
		return
	}

	genesis := bc.DeserializeBlock(res.Data)
	if genesis == nil {
		return
	}

	db, err := sql.Open("sqlite3", filename) 
	if err != nil {
		return
	}

	defer db.Close()
	_, err = db.Exec(bc.CREATE_TABLE)
	chain := &bc.BlockChain {
		DB: db,
	}
	chain.AddBlock(genesis)

	for i := uint64(1); i < num; i++ {
		res := nt.Send(address, &nt.Package{
			Option: GET_BLOCK,
			Data: fmt.Sprintf("&d", i),
		})
		if res == nil {
			return
		}

		block := bc.DeserializeBlock(res.Data)
		if block == nil {
			return
		}

		if block.IsValid(chain) {
			return
		}

		chain.AddBlock(block)
	}

	Mutex.Lock()
	Chain.DB.Close()
	os.Remove(Filename)
	copyFile(filename, Filename)
	Chain = bc.LoadChain(Filename)
	Block = bc.NewBlock(User.Address(), Chain.LastHash())
	Mutex.Unlock()

	if IsMining {
		BreakMining <- true
		IsMining = false
	}
}

func pushBlockToNet(block *bc.Block) {
	var (
		sblock = bc.SerializeBlock(block)
		msg = Serve + SEPARATOR + fmt.Sprintf("%d", Chain.Size()) + SEPARATOR + sblock
	)

	for _, addr := range Addresses {
		go nt.Send(addr, &nt.Package{
			Option: ADD_BLOCK,
			Data: msg,
		})
	}
}
	

func selectBlock(chain *bc.BlockChain, i int) string {
	var sblock string
	row := chain.DB.QueryRow("SELECT Block FROM BlockChain WHERE Id=$1", i+1)
	row.Scan(&sblock)
	return sblock
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out. Close()
}