package blockchain

import (
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/sammyklan3/finality/blockchain/server/dtos"
	"github.com/sammyklan3/finality/blockchain/utils"
	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

var (
	MAX_BLOCK_SIZE int = 10 // FIX: Set to higher number in production

	ErrMinAmount error = fmt.Errorf("Transaction amount cannot be less than minimum amount %v", MIN_TRANSACTION_AMOUNT)
	ErrBlockFull error = fmt.Errorf("Block full. Please create a new block")
)

func init() {
	err := utils.LoadEnv()
	if err != nil {
		log.Printf("Error loading environment variables; %v\n", err)
	}

	var maxBlockSize int
	maxBlockSize, err = strconv.Atoi(os.Getenv("MAX_BLOCK_SIZE"))
	if err != nil {
		// Set default MAX_BLOCK_SIZE value
		maxBlockSize = 10
	}

	MAX_BLOCK_SIZE = maxBlockSize
}

type Block struct {
	mu sync.RWMutex

	Id            uint32              `json:"id"`
	PrevBlockId   uint32              `json:"prev_block_id"`
	PrevBlockHash []byte              `json:"prev_hash"`
	Owner         string              `json:"owner"`
	Transactions  []SignedTransaction `json:"transactions"`
}

func NewBlock(prevBlockId uint32, prevBlockHash []byte, blockId uint32, owner string) (*Block, error) {
	if err := dtos.ValidateName(owner); err != nil {
		return nil, err
	}

	b := Block{
		mu: sync.RWMutex{},

		Id:            blockId,
		PrevBlockId:   prevBlockId,
		PrevBlockHash: prevBlockHash,
		Owner:         owner,
		Transactions:  []SignedTransaction{},
	}
	return &b, nil
}

func (b *Block) AddTransaction(t Transaction, privateKey ecdsa.PrivateKey) error {
	if len(b.Transactions) > MAX_BLOCK_SIZE {
		return ErrBlockFull
	}
	if err := t.Valid(); err != nil {
		return err
	}

	signature, err := t.Sign(privateKey)
	if err != nil {
		return err
	}
	signedTransaction := SignedTransaction{
		Transaction: t,
		Signature:   signature,
	}

	b.mu.Lock()
	b.Transactions = append(b.Transactions, signedTransaction)
	b.mu.Unlock()

	return nil
}

func (b *Block) hashBlock() ([]byte, error) {
	blockBytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	hash := md5.Sum(blockBytes)
	return hash[:], nil
}

// Signs a Block using the provided privateKey
// Returns a hash of the block, signature and an error (if one occurred)
func (b *Block) signBlock(privateKey *ecdsa.PrivateKey) ([]byte, []byte, error) {
	if privateKey == nil {
		return nil, nil, fmt.Errorf("NIL private key")
	}

	digest, err := b.hashBlock()
	if err != nil {
		return nil, nil, err
	}
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest)
	if err != nil {
		return nil, nil, err
	}
	return digest, signature, nil
}

func (b *Block) VerifyBlock(pubKey []byte, sig []byte) error {
	digest, err := b.hashBlock()
	if err != nil {
		return err
	}

	publicKey, err := mcrypto.DecodeECDSAPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("Error decoding ECDSA public key; %v", err)
	}

	ok := ecdsa.VerifyASN1(publicKey, digest, sig)
	if !ok {
		return fmt.Errorf("Invalid signature")
	}
	return nil
}

func (b *Block) String() string {
	return fmt.Sprintf("\nBlock<id=%v, owner=%v>\n", b.Id, b.Owner)
}
