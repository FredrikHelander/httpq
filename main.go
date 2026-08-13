package main

import (
	"log"
	"net/http"
)

func main() {
	httpQ := NewHTTPQ()

	tlsConfig, err := generateTLSConfig()
	if err != nil {
		log.Fatalf("failed to generate TLS config: %v", err)
	}

	server := &http.Server{
		Addr:      ":24744",
		Handler:   httpQ.Handler(),
		TLSConfig: tlsConfig,
	}

	log.Println("Starting HTTPS server on https://localhost:24744...")
	log.Fatal(server.ListenAndServeTLS("", ""))
}
