package main

import (
	"flag"
	"log"
	"os"

	"github.com/nicexiaonie/number-dispenser/internal/server"
)

func main() {
	// Parse command line flags
	addr := flag.String("addr", ":6380", "Server address to listen on")
	dataDir := flag.String("data", "./data", "Directory for data persistence")
	flag.Parse()

	// Create data directory if not exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create and start server
	srv, err := server.NewServer(*addr, *dataDir)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Println("Starting Number Dispenser Server...")
	log.Printf("Address: %s", *addr)
	log.Printf("Data Directory: %s", *dataDir)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
