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
}

func NewAPIServer(bc *core.Blockchain) *APIServer {
	return &APIServer{
		Blockchain: bc,
		server: &http.Server{
			Addr: ":8080",
		},
	}
}

func (s *APIServer) Start() error {
	http.HandleFunc("/blocks", s.handleGetBlocks)
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
