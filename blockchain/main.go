package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/sammyklan3/finality/blockchain/api"
	"github.com/sammyklan3/finality/blockchain/core"
)

func main() {
	chain := core.NewBlockchain()
	fmt.Println("Genesis block created:")
	fmt.Println(chain.LatestBlock())

	apiServer := api.NewAPIServer(chain)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := apiServer.Start(); err != nil {
			fmt.Println("API server error:", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down Finality node...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		fmt.Println("Error during shutdown:", err)
	} else {
		fmt.Println("Finality node stopped cleanly.")
	}
}
