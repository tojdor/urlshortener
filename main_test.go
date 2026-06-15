package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaveAndGet(t *testing.T) {
	//Arrange
	want := "https://go.dev"
	storage := &Storage{
		Codes: make(map[string]string),
	}
	service := &Service{
		Storage: storage,
	}

	//Act
	code, err := service.SaveInStorage(want)

	if err != nil {
		t.Fatalf("error while saving : %v", err)
	}

	got, err := service.GetOriginalURL(code)
	if err != nil {
		t.Fatalf("error while returning url: %v", err)
	}

	//Assert
	if got != want {
		t.Errorf("GetOriginalURL() = %s; want %s", got, want)
	}
}

func TestPostHandler(t *testing.T) {
	storage := &Storage{Codes: make(map[string]string)}
	service := &Service{Storage: storage}
	handler := &Handler{Service: service}
	var p PostResponse

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", handler.PostHandler)

	body := strings.NewReader(`{"url":"https://go.dev"}`)
	req := httptest.NewRequest("POST", "/shorten", body)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Errorf("bad status: %v", res.Code)
	}

	err := json.NewDecoder(res.Body).Decode(&p)
	if err != nil {
		t.Fatalf("cannot read result body: %v", err)
	}

	if !strings.HasPrefix(p.URL, "http://localhost:8080/") {
		t.Errorf("bad address: %v", p.URL)
	}

}

func TestRedirect(t *testing.T) {
	storage := &Storage{Codes: make(map[string]string)}
	service := &Service{Storage: storage}
	handler := &Handler{Service: service}

	knownCode, err := service.SaveInStorage("https://go.dev")
	if err != nil {
		t.Fatalf("error while saving in storage: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", handler.RedirectHandler)

	tests := []struct {
		name     string
		code     string
		wantCode int
	}{
		{"существующий код", knownCode, http.StatusFound},
		{"несуществующий код", "zzzz", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tc.code, nil)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)

			if res.Code != tc.wantCode {
				t.Errorf("status = %d; want %d", res.Code, tc.wantCode)
			}
		})
	}

}

type failingStorage struct{}

func (failingStorage) SaveURLCode(code, url string) error {
	return errors.New("db is down")
}

func (failingStorage) GetURLByCode(code string) (string, error) {
	return "", errors.New("db is down")
}

func TestRedirectInternalError(t *testing.T) {
	storage:=&failingStorage{}
	service:=&Service{Storage: storage}
	handler:=&Handler{Service: service}

	mux:=http.NewServeMux()
	mux.HandleFunc("GET /{code}", handler.RedirectHandler)

	req:=httptest.NewRequest("GET", "/zzzz", nil)
	res:=httptest.NewRecorder()

	mux.ServeHTTP(res, req)
	if res.Code!=http.StatusInternalServerError{
		t.Errorf("status = %d; want %d", res.Code, http.StatusInternalServerError)
	}
}
