package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func neededFormFields() []string {
	return []string{"base-curr", "target-curr", "start-date", "end-date"}
}

func startPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/index.html")
}

func getResource(w http.ResponseWriter, r *http.Request) {
	params := make(map[string]string)
	for _, field := range neededFormFields() {
		p := r.URL.Query().Get(field)

		if p == "" {
			err := fmt.Sprintf("Missing form field %s", field)
			http.Error(w, err, http.StatusBadRequest)
			return
		} else {
			params[field] = p
		}
	}

	fmt.Println(params)
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", startPage)
	r.Get("/api", getResource)

	http.ListenAndServe(":8080", r)
}
