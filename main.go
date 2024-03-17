package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type FrankfurterResponse struct {
	Amount    float64                       `json:"amount"`
	Base      string                        `json:"base"`
	StartDate string                        `json:"start_date"`
	EndDate   string                        `json:"end_date"`
	Rates     map[string]map[string]float64 `json:"rates"`
}

func (fr *FrankfurterResponse) get(w http.ResponseWriter, s string, e string, b string, t string) error {
	var date string
	if s == e {
		date = s
	} else {
		date = fmt.Sprintf("%s..%s", s, e)
	}

	reqURL := fmt.Sprintf("https://api.frankfurter.app/%s?from=%s&to=%s", date, b, t)
	resp, err := http.Get(reqURL)

	if err != nil {
		errMsg := fmt.Sprintf("Error fetching from %s: %v", reqURL, err)
		http.Error(w, errMsg, resp.StatusCode)
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(fr)
	if err != nil {
		errMsg := fmt.Sprintf("Server errored while processing data from %s", reqURL)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return err
	}

	return nil
}

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
	s := params["start-date"]
	e := params["end-date"]
	b := params["base-curr"]
	t := params["target-curr"]

	frResp := new(FrankfurterResponse)
	err := frResp.get(w, s, e, b, t)
	if err != nil {
		return
	}

	fmt.Printf("%+v\n", *frResp)
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
