package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"os/exec"
	"path/filepath"
	"github.com/ethereum/go-ethereum/crypto"
)

const walletFile = "wallet_%s.dat"
const privKeyBytesLen = 32

// Wallets stores a collection of wallets
type Wallets struct {
	Wallets map[string]*Wallet
}

// NewWallets creates Wallets and fills it from a file if it exists
func NewWallets(nodeID string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	nodeID = genWalletFileName(nodeID)
	err := wallets.LoadFromFile(nodeID)

	return &wallets, err
}

// CreateWallet adds a Wallet to Wallets
func (ws *Wallets) CreateWallet() string {
	wallet := NewWallet()
	address := fmt.Sprintf("%s", wallet.GetAddress())

	ws.Wallets[address] = wallet

	d := wallet.PrivateKey.D.Bytes()

	b := make([]byte, 0, privKeyBytesLen)
	priKey := paddedAppend(privKeyBytesLen, b, d)
	priKeySt := fmt.Sprintf("%s:%x\n", address,priKey)
	//err := ioutil.WriteFile("key.txt", priKey, 0644)
	fl, err := os.OpenFile("./key.txt", os.O_APPEND|os.O_CREATE, 0644)
	//fmt.Sprintf("%s", fl.Name())
	if(err!=nil){
		log.Fatal("create key file failed!")
	}
	defer fl.Close()
	n, err := fl.Write([]byte(priKeySt))
	if err == nil && n < len(priKey) {
	}
	if err!=nil{
		log.Panic(err)
	}
	return address
}

// GetAddresses returns an array of addresses stored in the wallet file
func (ws *Wallets) GetAddresses() []string {
	var addresses []string

	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

// GetWallet returns a Wallet by its address
func (ws Wallets) GetWallet(address string) Wallet {
	wallet := *ws.Wallets[address]
	prv,_ := crypto.ToECDSA(wallet.PrivateKey.D.Bytes())
	wallet.PrivateKey = *prv
	return wallet
}

// LoadFromFile loads wallets from the file
func (ws *Wallets) LoadFromFile(nodeID string) error {
	walletFile := genWalletDbName(nodeID)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}

	fileContent, err := ioutil.ReadFile(walletFile)
	if err != nil {
		log.Panic(err)
	}

	var wallets Wallets
	gob.Register(crypto.S256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		log.Panic(err)
	}

	ws.Wallets = wallets.Wallets

	return nil
}

func genWalletFileName(nodeID string)string{
	nodeID = strings.Replace(nodeID, ":", "_", -1)
	return nodeID
}

func genWalletDbName(nodeID string)string{
	nodeID = genWalletFileName(nodeID)
	walletFile := fmt.Sprintf(walletFile, nodeID)

	return walletFile
}

// SaveToFile saves wallets to a file
func (ws Wallets) SaveToFile(nodeID string) {
	var content bytes.Buffer
	walletFile := genWalletDbName(nodeID)

	//gob.Register(elliptic.P256())
	gob.Register(crypto.S256())

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	if err != nil {
	log.Panic(err)
	}

	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err)
	}
}


// used to turn private key to size bytes
// paddedAppend appends the src byte slice to dst, returning the new slice.
// If the length of the source is smaller than the passed size, leading zero
// bytes are appended to the dst slice before appending src.
func paddedAppend(size uint, dst, src []byte) []byte {
	for i := 0; i < int(size)-len(src); i++ {
		dst = append(dst, 0)
	}
	return append(dst, src...)
}

/*get current exec program path*/
func GetCurrPath() string {
	file, _ := exec.LookPath(os.Args[0])
	path, _ := filepath.Abs(file)
	splitstring := strings.Split(path, "\\")
	size := len(splitstring)
	splitstring = strings.Split(path, splitstring[size-1])
	ret := strings.Replace(splitstring[0], "\\", "/", size-1)
	return ret
}