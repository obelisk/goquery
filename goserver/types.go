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

type apiRequest struct {
	NodeKey string `json:"node_key"`
}

type distributedResponse struct {
	Queries  map[string]json.RawMessage `json:"queries"`
	Statuses map[string]int             `json:"statuses"`
	NodeKey  string                     `json:"node_key"`
}

type responseQuery struct {
	Rows     json.RawMessage
	Status   string
	SQLQuery string
}
