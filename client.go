package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	bc "github.com/sergeyHudzenko/go-blockchain/blockchain"
	nt "github.com/sergeyHudzenko/go-blockchain/network"
)

var (
	Addresses []string
	User *bc.User
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
	)

	var (
		addrExist = false
		userNewExist = false
		userLoadExist = false
	)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
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

	if !addrExist || !(userLoadExist || userNewExist) {
		panic("Failed 2")
	}

	err := json.Unmarshal([]byte(readFile(addrStr)), &Addresses)
	if err != nil {
		panic("Failed 3")
	}
	
	if len(Addresses) == 0 {
		panic("Failed 4")
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

func handleClient() {
	var (
		message string
		splited []string
	)

	for {
		message = inputString("> ")
		splited = strings.Split(message, " ")
		switch splited[0] {
		case "/exit":
			os.Exit(0)
		case "/user":
			if len(splited) < 2 {
				fmt.Println("len(user) < 2")
				continue
			}

			switch splited[1] {
			case "address": userAddress()
			case "purse": userPurse()
			case "balance": userBalance()

			}

		case "/chain":
			if len(splited) < 2 {
				fmt.Println("len(chain) < 2")
				continue
			}

			switch splited[1] {
				case "print": chainPrint()
				case "tx": chainTX(splited[1:])
				case "balance": chainBalance(splited[1:])
	
				}
		default:
			fmt.Println("Undefined command\n")
		}
	}
}

func inputString(begin string) string {
	fmt.Print(begin)
	msg, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.Replace(msg, "\n", "", 1)
}

func userAddress() {
	fmt.Println("Address:", User.Address(), "\n")
}

func userPurse() {
	fmt.Println("Purse:", User.Purse(), "\n")
}

func userBalance() {
	printBalance(User.Address())
}

func chainPrint() {
	for i := 0; ; i++ {
		res := nt.Send(Addresses[0], &nt.Package{
			Option: GET_BLOCK,
			Data: fmt.Sprintf("%d", i),
		})
		if res == nil || res.Data == "" {
			break
		}
		fmt. Printf ("[%d] => %s\n", i+1, res.Data)
	}
	fmt.Println()
}

func chainTX(splited []string) {
	if len(splited) < 3 {
		fmt.Println("len(splited) < 3\n")
		return
	}

	num, err := strconv.Atoi(splited[2])
	if err != nil {
		fmt.Println("strconv error\n")
		return
	}

	for _, addr := range Addresses {
		res := nt.Send(addr, &nt.Package{
			Option: GET_LHASH,
		})
	
		if res == nil {
			continue
		}
	
	tx := bc.NewTransaction(User, bc.Base64Decode(res.Data), splited[1], uint64(num))
	if tx == nil {
		fmt.Println("tx is null\n")
		return
	}

	res = nt.Send(addr, &nt.Package{
		Option: ADD_TRNSX,
		Data: bc.SerializeTX(tx),
	})
	
	fmt.Println("RES", res)

	if res == nil {
		continue
	}

	if res.Data  == "ok" {
		fmt.Printf("ok:	(%s)\n", addr)
	} else { 
		fmt.Printf("fail: (%s)\n", addr) 
	}
}

	fmt.Println()
}

func chainBalance(splited []string) {
	if len(splited) != 2 {
		fmt. Println("len(splited) |= 2\n")
		return
	}

	printBalance(splited [1])
}

func printBalance(address string) {
	for _, addr := range Addresses {
		res := nt.Send(addr, &nt.Package{
			Option: GET_BLNCE,
			Data: address,
		})

		if res == nil {
			continue
		}

		fmt.Printf("Balance (%s): %s coins\n", addr, res.Data)
	}

	fmt.Println()
}

func main() {
	handleClient()
}