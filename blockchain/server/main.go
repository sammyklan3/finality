package main

// The server is going to be responsible for receiving blocks from
// clients, verifying all transactions within that block and appending
// it to the blockchain.
// Before appending to the blockchain, the server will first replicate
// any received blocks among several interconnected peers. This arhitecture
// ensures availability; such that even if one node fails, the rest of the nodes
// on the cluster can continue serving client requests.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sammyklan3/finality/blockchain"
	"github.com/sammyklan3/finality/blockchain/server/database"
	"github.com/sammyklan3/finality/blockchain/server/dtos"
)

var (
	b *blockchain.Blockchain

	// Models
	organizations *database.OrganizationsTable = database.NewOrganizationTable()
)

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "Close")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type registerRequest struct {
	dtos.Organization
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		message := fmt.Sprintf("Error parsing form data; %v\n", err)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": message})
		return
	}

	// get uploaded public_key file from form input
	file, _, err := r.FormFile("public_key")
	if err != nil {
		log.Printf("Error parsing uploaded file 'public_key'; %v\n", err)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing uploaded file 'public_key'"})
		return
	}
	defer file.Close()

	publicKeyBytes, err := io.ReadAll(file)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error reading uploaded file 'public_key'"})
		return
	}

	req := registerRequest{
		Organization: dtos.Organization{
			AccountName: strings.ToUpper(r.FormValue("account_name")),
			PublicKey:   publicKeyBytes,
		},
	}

	// validate register request
	if err := req.Validate(); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// check if organization already exists; done to avoid duplicate registrations
	exists := organizations.Exists(req.AccountName)
	if exists {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "Organization already exists"})
		return
	}

	err = organizations.Create(req.Organization)
	if err != nil {
		log.Printf("Error creating organization; %v\n", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error creating organization"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Organization created successfully"})
}

func receiveBlockHandler(w http.ResponseWriter, r *http.Request) {
	var signedBlock blockchain.SignedBlock

	err := json.NewDecoder(r.Body).Decode(&signedBlock)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid signedBlock structure"})
		return
	}

	// Get signedBlock owner if exists
	org, err := organizations.Get(signedBlock.Owner)
	if err != nil {
		message := fmt.Sprintf("Block owner %v does not exist", signedBlock.Owner)
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": message})
		return
	}

	err = b.AddBlock(&signedBlock, org.PublicKey, signedBlock.Signature)
	if err != nil {
		message := "Error appending signedBlock to blockchain "
		log.Println(message, err)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": message})
		return
	}

	log.Printf("Current block; %v\n", signedBlock.Id)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Good block"})
}

func main() {
	wallet, err := blockchain.GetOrCreateWallet("GLOB_BLOCKCHAIN")
	if err != nil {
		log.Fatalf("Error creating server wallet; %v\n", err)
	}

	b, err = blockchain.NewBlockchain(*wallet)
	if err != nil {
		log.Fatalf("Error creating server blockchain; %v\n", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/auth/register", registerHandler)
	r.Post("/blocks", receiveBlockHandler)

	address := "localhost:5000"
	log.Printf("Starting blockchain server on address %v...\n", address)

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatalf("Error starting web server; %v\n", err)
	}
}
