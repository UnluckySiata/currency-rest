package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rmg/iso4217"
)

const bitSize = 64

type FormInfo struct {
	Today string
}

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
	Min      string
	Max      string
	Mean     string
	MinDate  string
	MaxDate  string
	DateFrom string
	DateTo   string
	CurrFrom string
	CurrTo   string
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

		var mean float64
		if mid > 0.0 {
			mean = (low + mid + high) / 3.0
		} else {
			mean = (low + high) / 2.0
		}
		sum += mean
		responses++

		if mean < minVal {
			minVal = mean
			minDate = date
		}
        if mean > maxVal {
			maxVal = mean
			maxDate = date
		}
	}
	res.Max = fmt.Sprintf("%.5f", maxVal)
	res.Min = fmt.Sprintf("%.5f", minVal)
	res.MinDate = minDate
	res.MaxDate = maxDate
	res.Mean = fmt.Sprintf("%.5f", sum/float64(responses))
}

func startPage(w http.ResponseWriter, r *http.Request) {
	y, m, d := time.Now().Date()
	var mm, dd string
	mm = fmt.Sprint(int(m))
	dd = fmt.Sprint(d)

	if m < 10 {
		mm = "0" + mm
	}
	if d < 10 {
		dd = "0" + dd
	}

	fi := FormInfo{
		Today: fmt.Sprintf("%d-%s-%s", y, mm, dd),
	}
	tmpl := template.Must(template.ParseFiles("html/index.html"))
	tmpl.Execute(w, fi)
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
	b := strings.ToUpper(params["base-curr"])
	t := strings.ToUpper(params["target-curr"])

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

	code, minor := iso4217.ByName(b)
	if code == 0 && minor == 0 {
		errMsg := fmt.Sprintf("%s passed as base-curr is not a valid ISO-4217 currency", b)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	code, minor = iso4217.ByName(t)
	if code == 0 && minor == 0 {
		errMsg := fmt.Sprintf("%s passed as target-curr is not a valid ISO-4217 currency", t)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	frResp := new(FrankfurterResponse)
	err = frResp.get(w, s, e, b, t)
	if err != nil {
		return
	}
	if len(frResp.Rates) == 0 {
		http.Error(w, "Frankfurter api resource not found", http.StatusNotFound)
		return
	}

	ecResp := new(EconomiaResponse)
	err = ecResp.get(w, s, e, b, t, daysBetween)
	if err != nil {
		return
	}
	if len(*ecResp) == 0 {
		http.Error(w, "Economia api resource not found", http.StatusNotFound)
		return
	}

	res := new(Result)
	res.DateFrom = s
	res.DateTo = e
	res.CurrFrom = b
	res.CurrTo = t

	res.calculate(frResp, ecResp, t)

	tmpl := template.Must(template.ParseFiles("html/result.html"))
	tmpl.Execute(w, res)
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
