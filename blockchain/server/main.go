// The server is going to be responsible for receiving blocks from
// clients, verifying all transactions within that block and appending
// it to the blockchain.
// Before appending to the blockchain, the server will first replicate
// any received blocks among several interconnected peers. This arhitecture
// ensures availability; such that even if one node fails, the rest of the nodes
// on the cluster can continue serving client requests.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sammyklan3/finality/blockchain/server/database"
	"github.com/sammyklan3/finality/blockchain/server/dtos"
)

var (
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
			OrgName:   r.FormValue("org_name"),
			PublicKey: publicKeyBytes,
		},
	}

	// validate register request
	if err := req.Validate(); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// check if organization already exists; done to avoid duplicate registrations
	exists := organizations.Exists(req.OrgName)
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

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/auth/register", registerHandler)

	address := "localhost:5000"
	log.Printf("Starting blockchain server on address %v...\n", address)

	err := http.ListenAndServe(address, r)
	if err != nil {
		log.Fatalf("Error starting web server; %v\n", err)
	}
}
