package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Begin goquery APIs
func checkHost(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	fmt.Printf("CheckHost call for: %s\n", r.FormValue("uuid"))

	// New DBWrapper Code
	_, host, err := db.GetHostInfo(uuid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Database Error: %s\n", err)
		return
	}
	renderedHost, err := json.Marshal(host)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Marshal Error: %s\n", err)
		return
	}
	fmt.Fprintf(w, "%s", renderedHost)
}

func scheduleQuery(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	sentQuery, err := json.Marshal(r.FormValue("query"))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Printf("ScheduleQuery call for: %s with query: %s\n", uuid, string(sentQuery))
	nodeKey, _, err := db.GetHostInfo(uuid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	query := Query{
		Name:   randomString(64),
		Query:  string(sentQuery),
		Status: "Pending",
	}

	queryMap[nodeKey][query.Name] = query
	fmt.Fprintf(w, "{\"queryName\" : \"%s\"}", query.Name)
}

func fetchResults(w http.ResponseWriter, r *http.Request) {
	queryName := r.FormValue("queryName")
	fmt.Printf("Fetching Results For: %s\n", queryName)
	// Yes I know this is really slow. For testing it should be fine
	// but I will fix this architecture later if needed
	// The real solution will be to use a better backing store like postgres
	for _, queries := range queryMap {
		if query, ok := queries[queryName]; ok {
			bytes, err := json.MarshalIndent(&query, "", "\t")
			if err != nil {
				fmt.Printf("Could not encode query result: %s\n", err)
				fmt.Fprintf(w, "Could not encode query result: %s\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Write(bytes)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// End goquery APIs
