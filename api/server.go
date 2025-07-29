package api

import (
	"context"
	"fmt"
	"net/http"
	"finality/core"
	"encoding/json"
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

	fmt.Println("API server listening on http://localhost:8080")
	return s.server.ListenAndServe()
}

func (s *APIServer) Shutdown(ctx context.Context) error {
	fmt.Println("Shutting down API server...")
	return s.server.Shutdown(ctx)
}

func (s *APIServer) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Blockchain.Blocks)
}

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
	json.NewDecoder(r.Body).Decode(&req)

	sig, err := s.Wallet.Sign([]byte(req.Message))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"signature": fmt.Sprintf("%x", sig),
	})
}

func (s *APIServer) handleWalletVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sigBytes := make([]byte, 64)
	_, err := fmt.Sscanf(req.Signature, "%x", &sigBytes)
	if err != nil {
		http.Error(w, "Invalid signature format", http.StatusBadRequest)
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
