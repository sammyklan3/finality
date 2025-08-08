package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sammyklan3/finality/blockchain/core"
)

type APIServer struct {
	Blockchain *core.Blockchain
	server     *http.Server
	Wallet     *core.Wallet
}

func NewAPIServer(bc *core.Blockchain) *APIServer {
	wallet := core.NewWallet()
	return &APIServer{
		Blockchain: bc,
		Wallet:     wallet,
		server: &http.Server{
			Addr: ":8080",
		},
	}
}

func (s *APIServer) Start() error {
    http.HandleFunc("/blocks", s.handleGetBlocks)
    http.HandleFunc("/wallet/address", s.handleWalletAddress)
    http.HandleFunc("/wallet/sign", s.handleWalletSign)
    http.HandleFunc("/wallet/verify", s.handleWalletVerify)
    http.HandleFunc("/wallet/keys", s.handleWalletKeys)
    http.HandleFunc("/wallet/new", s.handleNewWallet)
    http.HandleFunc("/wallet/import", s.handleImportWallet)

    fmt.Println("API server listening on http://localhost:8080")
    return s.server.ListenAndServe()
}


func (s *APIServer) Shutdown(ctx context.Context) error {
	fmt.Println("Shutting down API server...")
	return s.server.Shutdown(ctx)
}

// ================= Wallet Handlers =================

func (s *APIServer) handleWalletAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"address": s.Wallet.GetAddress(),
	})
}

func (s *APIServer) handleWalletSign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	sig, err := s.Wallet.Sign([]byte(req.Message))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"signature": hex.EncodeToString(sig),
	})
}

func (s *APIServer) handleWalletVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		http.Error(w, "Invalid signature hex", http.StatusBadRequest)
		return
	}

	valid := s.Wallet.Verify([]byte(req.Message), sigBytes)

	json.NewEncoder(w).Encode(map[string]bool{
		"valid": valid,
	})
}

func (s *APIServer) handleWalletKeys(w http.ResponseWriter, r *http.Request) {
	priv, _ := s.Wallet.ExportPrivateKey()
	pub, _ := s.Wallet.ExportPublicKey()

	json.NewEncoder(w).Encode(map[string]string{
		"private_key": string(priv),
		"public_key":  string(pub),
	})
}

func (s *APIServer) handleNewWallet(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    s.Wallet = core.NewWallet()
    priv, _ := s.Wallet.ExportPrivateKey()
    pub, _ := s.Wallet.ExportPublicKey()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "address":     s.Wallet.GetAddress(),
        "private_key": string(priv),
        "public_key":  string(pub),
    })
}


func (s *APIServer) handleImportWallet(w http.ResponseWriter, r *http.Request) {
	keyPem, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read key", http.StatusBadRequest)
		return
	}

	wallet, err := core.ImportPrivateKey(keyPem)
	if err != nil {
		http.Error(w, "Invalid private key", http.StatusBadRequest)
		return
	}

	s.Wallet = wallet
	pub, _ := wallet.ExportPublicKey()

	json.NewEncoder(w).Encode(map[string]string{
		"address":    wallet.GetAddress(),
		"public_key": string(pub),
	})
}

// ================= Blockchain Handlers =================

func (s *APIServer) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Blockchain.Blocks)
}
