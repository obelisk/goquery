package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func checkHost(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	fmt.Printf("CheckHost call for: %s\n", r.FormValue("uuid"))

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
	query, err := json.Marshal(r.FormValue("query"))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _, err = db.GetHostInfo(uuid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	queryName, err := db.ScheduleQuery(uuid, string(query))
	if err != nil {
		fmt.Printf("[DBWrapper] Error Scheduling Query: %s\n", err)
	}

	fmt.Fprintf(w, "{\"queryName\" : \"%s\"}", queryName)
	fmt.Printf("ScheduleQuery call for: %s with query: %s\n", uuid, string(query))
}

func fetchResults(w http.ResponseWriter, r *http.Request) {
	queryName := r.FormValue("queryName")
	fmt.Printf("Fetching Results For: %s\n", queryName)

	result, completeStatus, err := db.FetchResults(queryName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: Standardize these codes
	if completeStatus == "Unknown" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	query := Query{}
	query.Name = queryName
	query.Result = []byte(result)
	query.Status = completeStatus

	bytes, err := json.MarshalIndent(&query, "", "\t")
	if err != nil {
		fmt.Printf("Could not encode query result: %s\n", err)
		fmt.Fprintf(w, "Could not encode query result: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
	return
}
