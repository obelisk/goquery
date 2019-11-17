package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func enroll(w http.ResponseWriter, r *http.Request) {
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
	fmt.Printf("Enrolled a host (%s) with node_key: %s\n", newHost.UUID, nodeKey)

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
	// Eventually this is where ATC code will be because that system in based
	// on the config system, not the distributed system
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
	// Eventually we should pass this off to a logging backend, though I don't
	// have any idea which yet
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

	renderedQueries := ""
	pendingQueries, err := db.GetPendingHostQueries(parsedRequest.NodeKey)
	if err != nil {
		fmt.Printf("[DBWrapper] Error getting host queries: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for name, query := range pendingQueries {
		renderedQueries += fmt.Sprintf("\"%s\" : %s,", name, query)
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

	// Decode request body, but keep queries results as json.RawMessage
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

	//responses := make(map[string]*responseQuery)
	for queryName, resultsRaw := range responseParsed.Queries {
		marshalledResults, err := json.Marshal(&resultsRaw)
		if err != nil {
			fmt.Printf("Could not re-encode JSON from osquery:%s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		db.PutPendingQueryResults(queryName, string(marshalledResults), responseParsed.NodeKey)
	}
}
