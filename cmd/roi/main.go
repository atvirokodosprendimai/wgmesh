package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Extract static files
	staticContent, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to extract static files: %v", err)
	}

	// Serve static files
	http.Handle("/", http.FileServer(http.FS(staticContent)))

	// Lead capture endpoint
	http.HandleFunc("/api/capture", handleLeadCapture)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	addr := ":" + port
	log.Printf("ROI Calculator server starting on %s", addr)
	log.Printf("Visit http://localhost%s to use the calculator", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleLeadCapture handles lead capture submissions
func handleLeadCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// In a production environment, you would:
	// 1. Validate email format
	// 2. Store in a database
	// 3. Send confirmation email
	// 4. Trigger analytics event

	// Log the lead capture
	log.Printf("Lead captured: %s at %s", email, time.Now().Format(time.RFC3339))

	// Return success
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true,"message":"Thank you! Your PDF is being generated."}`)
}

// startWithPort attempts to start the server on the specified port, falling back to alternatives
func startWithPort(port int) error {
	addr := ":" + strconv.Itoa(port)
	log.Printf("Attempting to start on port %d...", port)
	
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}
