package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type PostRequest struct {
	URL string `json:"url"`
}

type PostResponse struct {
	URL string `json:"short_url"`
}

var codes = make(map[string]string)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status: "ok",
	}

	b, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Encoding json error", http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if url, ok := codes[code]; ok {
		http.Redirect(w, r, url, http.StatusFound)
	} else {
		http.Error(w, "link not found", http.StatusNotFound)
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var p PostRequest
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		http.Error(w, "Decoding json error", http.StatusBadRequest)
		return
	}
	code := generateCode(p.URL)
	codes[code] = p.URL

	b, err := json.Marshal(PostResponse{
		URL: "http://localhost:8080/" + code,
	})
	if err != nil {
		http.Error(w, "Encoding json error", http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func generateCode(url string) string {
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	if _, ok := codes[string(code)]; ok {
		return generateCode(url)
	}

	return string(code)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /shorten", postHandler)
	mux.HandleFunc("GET /{code}", redirectHandler)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
