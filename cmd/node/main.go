package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"finality/core"
	"finality/api"
)

func main() {
	chain := core.NewBlockchain()
	fmt.Println("Genesis block created:")
	fmt.Println(chain.LatestBlock())

	apiServer := api.NewAPIServer(chain)

	// Create context that listens for interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			fmt.Println("API server error:", err)
		}
	}()

	// Wait for interrupt
	<-ctx.Done()
	fmt.Println("\nShutting down Finality node...")

	// Give server time to shut down gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		fmt.Println("Error during shutdown:", err)
	} else {
		fmt.Println("Finality node stopped cleanly.")
	}
}
