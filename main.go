package main

import (
	"flag"
	"log"
	"os"

	"llm-proxy/pkg/server"
	"llm-proxy/pkg/session"
)

func main() {
	addr := flag.String("addr", ":8090", "Listen address for the proxy")
	adminToken := flag.String("admin-token", os.Getenv("GHOSTPROXY_ADMIN_TOKEN"), "Admin token for session registry endpoints")
	flag.Parse()

	logger := log.New(os.Stderr, "[llm-proxy] ", log.LstdFlags)
	store := session.NewMemoryStore()
	srv := server.New(store, logger, *adminToken)

	logger.Printf("starting llm-proxy on %s", *addr)
	if err := srv.Run(*addr); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
