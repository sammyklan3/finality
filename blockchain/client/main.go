package main

// The client is going to be run on organizations servers.
// It will be responsible for collecting transactions from the database or API,
// building transactions into blocks and sending full blocks to the
// blockchain server.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/sammyklan3/finality/blockchain"
	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

var (
	remote string
)

func init() {
	flag.StringVar(&remote, "remote", "localhost:5000", "Remote server to send signed blocks to")
	flag.Parse()
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

func sendSignedBlocks(signedBlock blockchain.SignedBlock, address string) error {
	url := fmt.Sprintf("http://%v/blocks", address)
	fmt.Printf("Sending signed block to %v\n", url)

	var requestBody bytes.Buffer

	err := json.NewEncoder(&requestBody).Encode(signedBlock)
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

func main() {
	owner := randomChoice(organizations)
	if owner == nil {
		log.Fatalln("Error selecting random owner; NIL value. Please fill in organizations array with valid data")
	}

	wallet, err := blockchain.GetOrCreateWallet(*owner)
	if err != nil {
		log.Fatalf("Error creating client wallet; %v\n", err)
	}

	err = sendRegisterRequest(*wallet, remote)
	if err != nil {
		log.Fatalf("Error registering owner; %v\n", err)
	}

	b, err := blockchain.NewBlockchain(*wallet)
	if err != nil {
		log.Fatalf("Error generating new blockchain; %v\n", err)
	}

	signedBlocks := make(chan blockchain.SignedBlock, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We are going to run a goroutine that
	// is going to append signed blocks onto the blockchain

	go generateBlocks(ctx, b, signedBlocks)

	// Capture generated signed blocks and send to server for syncing
	for {
		signedBlock := <-signedBlocks

		err = sendSignedBlocks(signedBlock, remote)
		if err != nil {
			log.Fatalf("Error sending signed block to %v; %v\n", remote, err)
		}
	}

}
