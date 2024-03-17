package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const bitSize = 64

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

type EconomiaRecord struct {
	High      string `json:"high"`
	Low       string `json:"low"`
	Timestamp string `json:"timestamp"`
}

type EconomiaResponse []EconomiaRecord

func (er *EconomiaResponse) get(w http.ResponseWriter, s string, e string, b string, t string, c int64) error {
	s = strings.Replace(s, "-", "", -1)
	e = strings.Replace(e, "-", "", -1)

	reqURL := fmt.Sprintf("https://economia.awesomeapi.com.br/json/daily/%s-%s/%d?start_date=%s&end_date=%s", b, t, c, s, e)
	resp, err := http.Get(reqURL)

	if err != nil {
		errMsg := fmt.Sprintf("Error fetching from %s: %v", reqURL, err)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(er)
	if err != nil {
		errMsg := fmt.Sprintf("Server errored while processing data from %s", reqURL)
		http.Error(w, errMsg, http.StatusInternalServerError)
		fmt.Printf("e: %v\n", err)
		return err
	}

	return nil
}

func neededFormFields() []string {
	return []string{"base-curr", "target-curr", "start-date", "end-date"}
}

type Result struct {
	min     float64
	max     float64
	mean    float64
	minDate string
	maxDate string
}

func (res *Result) calculate(fr *FrankfurterResponse, er *EconomiaResponse, target string) {
	minVal := 100000.0
	maxVal := 0.0
	sum := 0.0
	responses := 0
	var minDate, maxDate string

	for _, e := range *er {
		t, _ := strconv.ParseInt(e.Timestamp, 10, bitSize)
		y, m, d := time.Unix(t, 0).Date()

		dd := fmt.Sprint(d)
		mm := fmt.Sprint(int(m))
		if d < 10 {
			dd = "0" + dd
		}
		if m < 10 {
			mm = "0" + mm
		}

		date := fmt.Sprintf("%d-%s-%s", y, mm, dd)
		f := fr.Rates[date]

		low, _ := strconv.ParseFloat(e.Low, bitSize)
		high, _ := strconv.ParseFloat(e.High, bitSize)
		mid := f[target]

		mean := (low + mid + high) / 3.0
		sum += mean
		responses++

		if mean < minVal {
			minVal = mean
			minDate = date
		} else if mean > maxVal {
			maxVal = mean
			maxDate = date
		}
	}
	res.max = maxVal
	res.min = minVal
	res.minDate = minDate
	res.maxDate = maxDate
	res.mean = sum / float64(responses)
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

	sd, err := time.Parse(time.DateOnly, s)
	if err != nil {
		http.Error(w, "Wrong start-date format", http.StatusBadRequest)
		return
	}

	ed, err := time.Parse(time.DateOnly, e)
	if err != nil {
		http.Error(w, "Wrong end-date format", http.StatusBadRequest)
		return
	}

	daysBetween := (ed.Unix() - sd.Unix()) / 86400
	if daysBetween < 0 {
		http.Error(w, "end-date can't be earlier than start-date", http.StatusBadRequest)
		return
	}

	frResp := new(FrankfurterResponse)
	err = frResp.get(w, s, e, b, t)
	if err != nil {
		return
	}

	ecResp := new(EconomiaResponse)
	err = ecResp.get(w, s, e, b, t, daysBetween)
	if err != nil {
		return
	}

	res := new(Result)
	res.calculate(frResp, ecResp, t)
	fmt.Printf("%+v\n", res)
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
