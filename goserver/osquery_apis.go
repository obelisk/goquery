package main

import (
	"fmt"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
)

// Begin osquery API endpoints
func enroll(w http.ResponseWriter, r *http.Request) {
	type osVersionInfo struct {
		Platform string `json:"platform"`
		Version  string `json:"version"`
	}
	type osqueryInfo struct {
		Version string `json:"version"`
	}
	type enrollSystemInfo struct {
		UUID         string `json:"uuid"`
		ComputerName string `json:"computer_name"`
	}
	type hostDetailsBody struct {
		SystemInfo    enrollSystemInfo `json:"system_info"`
		OsqueryInfo   osqueryInfo      `json:"osquery_info"`
		OsVersionInfo osVersionInfo    `json:"os_version"`
	}
	type enrollBody struct {
		EnrollSecret   string          `json:"enroll_secret"`
		HostIdentifier string          `json:"host_identifier"`
		HostDetails    hostDetailsBody `json:"host_details"`
	}

	parsedBody := enrollBody{}
	jsonBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Error reading request body: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(jsonBytes, &parsedBody)
	if err != nil {
		fmt.Printf("Error decoding request JSON: %s\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if parsedBody.EnrollSecret != ENROLL_SECRET {
		fmt.Printf("Host provided incorrrect secret: %s\n", parsedBody.EnrollSecret)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "{\"node_invalid\" : true}")
		return
	}
	nodeKey := randomString(32)
	fmt.Fprintf(w, "{\"node_key\" : \"%s\"}", nodeKey)
	newHost := Host{
		UUID:         parsedBody.HostDetails.SystemInfo.UUID,
		ComputerName: parsedBody.HostDetails.SystemInfo.ComputerName,
		Version:      parsedBody.HostDetails.OsqueryInfo.Version,
		Platform:     parsedBody.HostDetails.OsVersionInfo.Platform + "(" + parsedBody.HostDetails.OsVersionInfo.Version + ")",
	}
	// The configuration is overriding the host_identifier with something else so we
	// should definitely use that for indexing
	if parsedBody.HostIdentifier != "" {
		newHost.UUID = parsedBody.HostIdentifier
	}
	queryMap[nodeKey] = make(map[string]Query)
	fmt.Printf("Enrolled a host (%s) with node_key: %s\n", newHost.UUID, nodeKey)

	// New DBWrapper Code
	db.EnrollNewHost(nodeKey, newHost)
}

func isNodeKeyEnrolled(ar apiRequest) bool {
	enrolled, err := db.NodeKeyEnrolled(ar.NodeKey)
	if err != nil {
		fmt.Printf("[Error] Threw an error checking NodeKey enrollment: %s\n", err)
		return false
	}
	return enrolled
}

func httpRequestToAPIRequest(r *http.Request) (apiRequest, error) {
	parsedRequest := apiRequest{}
	jsonBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Could not read the body of an API request\n")
		return apiRequest{}, err
	}
	err = json.Unmarshal(jsonBytes, &parsedRequest)
	if err != nil {
		return apiRequest{}, err
	}
	return parsedRequest, nil
}

func config(w http.ResponseWriter, r *http.Request) {
	// This server is designed to test goquery so we don't push a config
	parsedRequest, err := httpRequestToAPIRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isNodeKeyEnrolled(parsedRequest) {
		fmt.Fprintf(w, "{\"schedule\":{}, \"node_invalid\" : true}")
		return
	}

	fmt.Fprintf(w, "{\"schedule\":{}, \"node_invalid\" : false}")
}

func log(w http.ResponseWriter, r *http.Request) {
	// This server is designed to test goquery so we don't do anything with the logs
}

func distributedRead(w http.ResponseWriter, r *http.Request) {
	parsedRequest, err := httpRequestToAPIRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isNodeKeyEnrolled(parsedRequest) {
		fmt.Fprintf(w, "{\"node_invalid\" : true}")
		return
	}

	// The check below should never fail. If it does we've really screwed up
	renderedQueries := ""
	if _, ok := queryMap[parsedRequest.NodeKey]; !ok {
		fmt.Fprintf(w, "{\"node_invalid\" : true}")
		fmt.Printf("This should never occur. A host is enrolled but not configured for distributed\n")
		return
	}
	for name, query := range queryMap[parsedRequest.NodeKey] {
		if query.Complete {
			continue
		}
		renderedQueries += fmt.Sprintf("\"%s\" : %s,", name, query.Query)
	}

	renderedQueries = strings.TrimRight(renderedQueries, ",")
	if len(renderedQueries) > 0 {
		fmt.Fprintf(w, "{\"queries\" : {%s}, \"accelerate\" : 300}", renderedQueries)
	} else {
		fmt.Fprintf(w, "{\"queries\" : {%s}}", renderedQueries)
	}
}

func distributedWrite(w http.ResponseWriter, r *http.Request) {
	jsonBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Could not read body: %s\n", err)
		return
	}

	type distributedResponse struct {
		Queries  map[string]json.RawMessage `json:"queries"`
		Statuses map[string]int             `json:"statuses"`
		NodeKey  string                     `json:"node_key"`
	}

	// Decode request body, but don't bother decoding the query results
	// These should be opaquely passed along when asked for
	responseParsed := distributedResponse{}
	if err := json.Unmarshal(jsonBytes, &responseParsed); err != nil {
		fmt.Printf("Could not parse body: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isNodeKeyEnrolled(apiRequest{NodeKey: responseParsed.NodeKey}) {
		fmt.Fprintf(w, "{\"node_invalid\" : true}")
		fmt.Printf("The host sending results is not enrolled\n")
		return
	}

	type responseQuery struct {
		Rows     json.RawMessage
		Status   string
		SQLQuery string
	}
	responses := make(map[string]*responseQuery)
	for queryName, resultsRaw := range responseParsed.Queries {
		sqlQuery := queryMap[responseParsed.NodeKey][queryName].Query
		responses[queryName] = &responseQuery{
			SQLQuery: sqlQuery,
			Rows:     resultsRaw,
		}
	}
	for queryName, statusCode := range responseParsed.Statuses {
		if statusCode == 0 {
			responses[queryName].Status = "Complete"
		} else {
			responses[queryName].Status = fmt.Sprintf("Status Code %d", statusCode)
		}
	}

	for queryName, response := range responses {
		queryMap[responseParsed.NodeKey][queryName] = Query{
			Query:    response.SQLQuery,
			Name:     queryName,
			Complete: true,
			Result:   response.Rows,
			Status:   response.Status,
		}
		fmt.Printf("Received and set query results for %s\n", queryName)
	}
}

// End osquery API endpoints
