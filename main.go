package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var ErrNotFound = errors.New("not found")

type HealthResponse struct {
	Status string `json:"status"`
}

type PostRequest struct {
	URL string `json:"url"`
}

type PostResponse struct {
	URL string `json:"short_url"`
}

type Storer interface {
	SaveURLCode(code string, url string) error
	GetURLByCode(code string) (string, error)
}

type Storage struct {
	mx    sync.Mutex
	Codes map[string]string
}

type PostgresStorage struct {
	pool *pgxpool.Pool
}

func (ps *PostgresStorage) SaveURLCode(code string, url string) error {
	_, err := ps.pool.Exec(context.Background(),
		"INSERT INTO links (code, url) VALUES($1, $2)", code, url)
	if err != nil {
		return err
	}
	return nil
}

func (ps *PostgresStorage) GetURLByCode(code string) (string, error) {
	var url string
	err := ps.pool.QueryRow(context.Background(),
		"SELECT url FROM links WHERE code = $1", code).Scan(&url)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return url, nil
}

func (st *Storage) SaveURLCode(code string, url string) error {
	st.mx.Lock()
	defer st.mx.Unlock()
	st.Codes[code] = url
	return nil
}

func (st *Storage) GetURLByCode(code string) (string, error) {
	st.mx.Lock()
	defer st.mx.Unlock()
	url, ok := st.Codes[code]
	if !ok {
		return "", ErrNotFound
	}

	return url, nil
}

type Service struct {
	Storage Storer
}

func (s *Service) GenerateCode(url string) string {
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}

	if _, err := s.Storage.GetURLByCode(string(code)); err == nil {
		return s.GenerateCode(url) //возможна редкая коллизия при гонке, не критично
	}
	return string(code)
}

func (s *Service) SaveInStorage(url string) (string, error) {
	code := s.GenerateCode(url)
	err := s.Storage.SaveURLCode(code, url)
	if err != nil {
		return "", err
	}
	return code, nil
}

func (s *Service) GetOriginalURL(code string) (string, error) {
	url, err := s.Storage.GetURLByCode(code)
	return url, err
}

type Handler struct {
	Service *Service
}

func (h *Handler) PostHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var p PostRequest
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		http.Error(w, "error while decoding", http.StatusBadRequest)
		return
	}

	code, err := h.Service.SaveInStorage(p.URL)
	if err != nil {
		http.Error(w, "error while saving in storage", http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(PostResponse{
		URL: "http://localhost:8080/" + code,
	})
	if err != nil {
		http.Error(w, "encoding request body error", http.StatusInternalServerError)
		return
	}

	w.Write(body)
}

func (h *Handler) RedirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		http.Error(w, "code is empty", http.StatusBadRequest)
		return
	}

	originalPath, err := h.Service.GetOriginalURL(code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, originalPath, http.StatusFound)
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("not load .env file")
	}

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("cant load database url")
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	err = pool.Ping(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	pgStorage := &PostgresStorage{
		pool: pool,
	}

	service := Service{
		Storage: pgStorage,
	}
	handler := Handler{
		Service: &service,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", handler.PostHandler)
	mux.HandleFunc("GET /{code}", handler.RedirectHandler)
	mux.HandleFunc("GET /health", handler.HealthHandler)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
