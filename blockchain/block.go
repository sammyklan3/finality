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
	"strings"
	"sync/atomic"

	"github.com/sammyklan3/finality/blockchain/server/dtos"
	"github.com/sammyklan3/finality/blockchain/utils"
	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

var (
	blockId        atomic.Uint32 = atomic.Uint32{}
	MAX_BLOCK_SIZE int           = 10 // FIX: Set to higher number in production

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
	Id            uint32              `json:"id"`
	PrevBlockId   uint32              `json:"prev_block_id"`
	PrevBlockHash string              `json:"prev_hash"`
	Owner         string              `json:"owner"`
	Transactions  []SignedTransaction `json:"transactions"`
}

func NewBlock(prevBlockId uint32, prevBlockHash, owner string) (*Block, error) {
	if err := dtos.ValidateName(owner); err != nil {
		return nil, err
	}

	return &Block{
		Id:            blockId.Add(1),
		PrevBlockId:   prevBlockId,
		PrevBlockHash: prevBlockHash,
		Owner:         owner,
		Transactions:  []SignedTransaction{},
	}, nil
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

	mu.Lock()
	b.Transactions = append(b.Transactions, signedTransaction)
	mu.Unlock()

	return nil
}

func (b *Block) Hash() ([]byte, error) {
	blockBytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	hash := md5.Sum(blockBytes)
	return hash[:], nil
}

func (b *Block) SignBlock(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("NIL private key")
	}

	digest, err := b.Hash()
	if err != nil {
		return nil, err
	}
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest)
	if err != nil {
		return nil, err
	}

	log.Printf("SignBlock %v;\n\tHash: %x;\n\tSignature: %x\n\tPrivateKey: %v\n\tPublicKey: %v\n", b.Id, digest, signature, privateKey, privateKey.PublicKey)
	return signature, nil
}

func (b *Block) VerifyBlock(pubKey []byte, sig []byte) error {
	digest, err := b.Hash()
	if err != nil {
		return err
	}

	publicKey, err := mcrypto.DecodeECDSAPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("Error decoding ECDSA public key; %v", err)
	}

	log.Printf("VerifyBlock %v;\n\tHash: %x;\n\tSignature: %x\n\tPublicKey: %v\n", b.Id, digest, sig, publicKey)

	ok := ecdsa.VerifyASN1(publicKey, digest, sig)
	if !ok {
		return fmt.Errorf("Invalid signature")
	}
	return nil
}

func (b *Block) String() string {
	header := fmt.Sprintf("Block<id=%v, owner=%v>", b.Id, b.Owner)

	var str strings.Builder
	str.WriteString(header)
	str.WriteString("\n")

	for _, t := range b.Transactions {
		str.WriteString(t.String())
		str.WriteString("\n")
	}
	return str.String()
}
