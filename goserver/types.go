package main

import "encoding/json"

type Host struct {
	UUID           string
	ComputerName   string
	HostIdentifier string
	Platform       string
	Version        string
}

type Query struct {
	Query    string
	Name     string
	Complete bool
	Result   json.RawMessage `json:"results"`
	Status   string          `json:"status"`
}

type apiRequest struct {
	NodeKey string `json:"node_key"`
}
