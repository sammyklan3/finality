// The client is going to be run on organizations servers.
// It will be responsible for collecting transactions from the database or API,
// building transactions into blocks and sending full blocks to the
// blockchain server.

package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sammyklan3/finality/blockchain"
	"github.com/sammyklan3/finality/blockchain/utils"
	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

var (
	organizations = []string{
		"JPMorgan Chase & Co.",
		"Goldman Sachs",
		"Bank of America",
		"Citigroup",
		"HSBC Holdings",
	}
	names = []string{
		"Alice Johnson",
		"Bob Smith",
		"Charlie Williams",
		"Diana Evans",
		"Ethan Brown",
		"Fiona Davis",
		"George Miller",
		"Hannah Wilson",
		"Ian Clark",
		"Julia Lewis",
		"Kevin Hall",
		"Lara Young",
		"Michael King",
		"Nora Scott",
		"Oliver Adams",
		"Paula Turner",
		"Quentin Baker",
		"Rachel Harris",
		"Samuel Allen",
		"Tina Martin",
	}
)

var (
	remote string
)

func init() {
	flag.StringVar(&remote, "remote", "localhost:5000", "Remote server to send signed blocks to")
	flag.Parse()
}

func randomChoice[T any](arr []T) *T {
	if len(arr) == 0 {
		return nil
	}
	index := rand.IntN(len(arr))
	return &arr[index]
}

func generateTransaction() (*blockchain.Transaction, error) {
	sender := randomChoice(names)
	receiver := randomChoice(names)
	if sender == nil || receiver == nil {
		return nil, fmt.Errorf("Error selecting random sender or receiver; NIL values. Please fill in the names array with valid data")
	}

	amount := rand.UintN(math.MaxUint16)

	t, err := blockchain.NewTransaction(*sender, *receiver, amount)
	if err != nil {
		return nil, fmt.Errorf("Error creating new transaction; %v", err)
	}
	return t, nil
}

func generateFullBlock(wallet blockchain.Wallet) (*blockchain.Block, error) {
	block, err := blockchain.NewBlock(0, "000000", wallet.Owner)
	if err != nil {
		return nil, fmt.Errorf("Error generating new block; %v", err)
	}

	privateKey, err := wallet.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("Error fetching wallet private key; %v\n", err)
	}

	for range blockchain.MAX_BLOCK_SIZE {
		t, err := generateTransaction()
		if err != nil {
			return nil, fmt.Errorf("Error generating random transaction; %v", err)
		}

		err = block.AddTransaction(*t, *privateKey)
		if err != nil {
			return nil, fmt.Errorf("Error adding transaction to block; %v", err)
		}
	}
	return block, nil
}

func printResponse(response *http.Response) {
	if response == nil {
		log.Println("NIL response")
		return
	}

	var responseBody map[string]string

	err := json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		log.Printf("Error decoding response body; %v\n", err)
		return
	}

	fmt.Println(response.Status)
	fmt.Println(responseBody)
	fmt.Println()
}

func sendFullBlock(block blockchain.Block, wallet blockchain.Wallet, address string) error {
	url := fmt.Sprintf("http://%v/blocks", address)
	fmt.Printf("Sending signed block to %v\n", url)

	privateKey, err := wallet.PrivateKey()
	if err != nil {
		return err
	}

	signedBlock, err := blockchain.NewSignedBlock(block, privateKey)
	if err != nil {
		return fmt.Errorf("Error signing block; %v", err)
	}

	var requestBody bytes.Buffer

	err = json.NewEncoder(&requestBody).Encode(signedBlock)
	if err != nil {
		return err
	}

	response, err := http.Post(url, "application/json", &requestBody)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	printResponse(response)
	return nil
}

func sendRegisterRequest(wallet blockchain.Wallet, address string) error {
	accountName := strings.TrimSpace(wallet.Owner)
	if accountName == "" {
		return fmt.Errorf("Account name cannot be empty")
	}

	fmt.Printf("Registering client %v with backend\n", accountName)

	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)

	// Send additional data with multipart/form-data
	err := multipartWriter.WriteField("account_name", accountName)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%v.pub", strings.ToLower(accountName))
	writer, err := multipartWriter.CreateFormFile("public_key", filename)
	if err != nil {
		return err
	}

	// Encode public key into PEM format b4 sending it
	privateKey, err := wallet.PrivateKey()
	if err != nil {
		return fmt.Errorf("Error acquiring wallet private key; %v", err)
	}

	pemEncodedPublicKey, err := mcrypto.EncodeECDSAPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("Error encoding ECDSA public key; %v", err)
	}
	writer.Write(pemEncodedPublicKey)

	// Close the multipartWriter to set the terminating boundary
	if err = multipartWriter.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("http://%v/auth/register", address)
	request, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	printResponse(response)
	return nil
}

// Tries reading gob saved wallet from file, or creates new wallet
// and writes it to the file if it does not exist within file
func getOrCreateWallet(owner string) (*blockchain.Wallet, error) {
	filename := filepath.Join(utils.SecretsDir, fmt.Sprintf("%v.wallet", owner))

	// Open file for read/write, create if not exists
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Try reading wallet from file
	var wallet blockchain.Wallet
	err = gob.NewDecoder(file).Decode(&wallet)
	if err == nil {
		return &wallet, nil // successfully decoded
	}

	// If file is empty (new), create a new wallet
	if errors.Is(err, io.EOF) {
		newWallet, err := blockchain.NewWallet(owner)
		if err != nil {
			return nil, fmt.Errorf("Error creating new wallet; %v", err)
		}

		// Reset file before writing
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		if err := file.Truncate(0); err != nil {
			return nil, err
		}

		enc := gob.NewEncoder(file)
		if err := enc.Encode(newWallet); err != nil {
			return nil, err
		}

		return newWallet, nil
	}

	// Any other decode error means corruption
	return nil, fmt.Errorf("failed to decode wallet: %w", err)
}

func createRandomWallet() (*blockchain.Wallet, error) {
	owner := randomChoice(organizations)
	if owner == nil {
		return nil, fmt.Errorf("Error selecting random owner; NIL value. Please fill in organizations array with valid data")
	}
	return getOrCreateWallet(*owner)
}

func main() {
	wallet, err := createRandomWallet()
	if err != nil {
		log.Fatalf("Error creating client wallet; %v\n", err)
	}

	err = sendRegisterRequest(*wallet, remote)
	if err != nil {
		log.Fatalf("Error registering owner; %v\n", err)
	}

	block, err := generateFullBlock(*wallet)
	if err != nil {
		log.Fatalf("Error generating full block; %v\n", err)
	}

	err = sendFullBlock(*block, *wallet, remote)
	if err != nil {
		log.Fatalf("Error sending signed block to %v; %v\n", remote, err)
	}

}
