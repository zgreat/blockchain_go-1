package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"strings"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"time"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"sync/atomic"
	"github.com/ethereum/go-ethereum/rlp"
	"os"
	."../boltqueue"
)

const subsidy = 50

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
	Timestamp     int64
	size atomic.Value
}
// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction
type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// IsCoinbase checks whether the transaction is coinbase
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// Serialize returns a serialized Transaction
func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	hash = sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

// Sign signs each input of a Transaction
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}

	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		dataToSign := fmt.Sprintf("%x\n", txCopy)

		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataToSign))
		if err != nil {
			log.Panic(err)
		}
		signature := append(r.Bytes(), s.Bytes()...)

		tx.Vin[inID].Signature = signature
		txCopy.Vin[inID].PubKey = nil
	}
}

// String returns a human-readable representation of a transaction
func (tx Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	for i, input := range tx.Vin {

		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}

	for i, output := range tx.Vout {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
	}
	    lines = append(lines, fmt.Sprintf("       Timestamp: %d", tx.Timestamp))

	return strings.Join(lines, "\n")
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	for _, vin := range tx.Vin {
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil})
	}

	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}

	var v = atomic.Value{}
	v.Store(common.StorageSize(0))
	txCopy := Transaction{tx.ID, inputs, outputs,tx.Timestamp,tx.size}
	tx.SetSize(uint64(len(tx.Serialize())))
	//txCopy.size.Store(tx.Size())

	return txCopy
}

// Verify verifies signatures of Transaction inputs
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}

	txCopy := tx.TrimmedCopy()
	curve := crypto.S256()

	for inID, vin := range tx.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		r := big.Int{}
		s := big.Int{}
		sigLen := len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen / 2)])
		s.SetBytes(vin.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen / 2)])
		y.SetBytes(vin.PubKey[(keyLen / 2):])

		dataToVerify := fmt.Sprintf("%x\n", txCopy)

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		if ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) == false {
			return false
		}
		txCopy.Vin[inID].PubKey = nil
	}

	return true
}

// NewCoinbaseTX creates a new coinbase transaction
func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		randData := make([]byte, subsidy)
		_, err := rand.Read(randData)
		if err != nil {
			log.Panic(err)
		}

		data = fmt.Sprintf("%x", randData)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout := NewTXOutput(subsidy, to)
	var v = atomic.Value{}
	v.Store(common.StorageSize(0))
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}, time.Now().Unix(),v}
	tx.ID = tx.Hash()
	tx.SetSize(uint64(len(tx.Serialize())))

	return &tx
}

// NewUTXOTransaction creates a new transaction
func NewUTXOTransaction(wallet *Wallet, to string, amount int, UTXOSet *UTXOSet) *Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	pubKeyHash := HashPubKey(wallet.PublicKey)
	acc, validOutputs := UTXOSet.FindSpendableOutputs(pubKeyHash, amount,false,nil)

	if acc < amount {
		log.Panic("ERROR: Not enough funds")
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}
		for _, out := range outs {
			input := TXInput{txID, out, nil, wallet.PublicKey}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	from := fmt.Sprintf("%s", wallet.GetAddress())
	outputs = append(outputs, *NewTXOutput(amount, to))
	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, from)) // a change
	}

	var v = atomic.Value{}
	v.Store(common.StorageSize(0))
	tx := Transaction{nil, inputs, outputs, time.Now().Unix(),v}
	tx.ID = tx.Hash()
	tx.SetSize(uint64(len(tx.Serialize())))
	UTXOSet.Blockchain.SignTransaction(&tx, wallet.PrivateKey)

	return &tx
}

// DeserializeTransaction deserializes a transaction
func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	if err != nil {
		log.Panic(err)
	}

	return transaction
}

func VeryfyFromToAddress(tx *Transaction) bool{
	if tx.IsCoinbase() {
		return true
	}
	if len(tx.Vout)>0 {
		for _, txin := range tx.Vin {
			if hex.EncodeToString(txin.PubKey) == hex.EncodeToString(tx.Vout[0].PubKeyHash) {
				//log.Panic("ERROR: Wallet from equal Wallet to is not valid")
				return false
			}
		}
	}
	return true
}

/*
// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}
*/

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil && size != common.StorageSize(0) {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

func (tx *Transaction) SetSize(c uint64) common.StorageSize {
	tx.size.Store(common.StorageSize(c))
	return  common.StorageSize(c)
}

func PendingIn(wallet Wallet,tx *Transaction){
	queueFile := fmt.Sprintf("%x_tx.db", wallet.GetAddress())
	txPQueue, err := NewPQueue(queueFile)
	if err != nil {
		log.Panic("create queue error",err)
	}
	//defer txPQueue.Close()
	defer os.Remove(queueFile)
	eqerr := txPQueue.Enqueue(1, NewMessageBytes(tx.Vin[0].Txid))
	if err != nil {
		log.Panic("Enqueue error",eqerr)
	}
	txPQueue.Close()
}

func VerifyTx(tx Transaction,bc *Blockchain)bool{
	if &tx == nil {
		fmt.Println("transaction %x is nil:",&tx.ID)
		return false
	}
	// ignore transaction if it's not valid
	// 1 have received longest chain among knownodes(full nodes)
	// 2 transaction have valid sign accounding to owner's pubkey by VerifyTransaction()
	// 3 utxo amount >= transaction output amount
	// 4 transaction from address not equal to address
	var valid1 = true
	var valid2 = true
	var valid3 = true
	if !bc.VerifyTransaction(&tx) {
		//log.Panic("ERROR: Invalid transaction:sign")
		valid1 = false
	}
	UTXOSet := UTXOSet{bc}
	if(tx.IsCoinbase()==false&&!UTXOSet.IsUTXOAmountValid(&tx)){
		//log.Panic("ERROR: Invalid transaction:amount")
		valid2 = false
	}
	valid3 = VeryfyFromToAddress(&tx)
	fmt.Printf("valid3  %s\n", valid3)

	if(valid1 && valid2 && valid3){
		return true
	}else{
		return false
	}
}