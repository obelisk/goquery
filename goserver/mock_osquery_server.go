package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"flag"
	"fmt"
	"net/http"
	"net/url"

	"github.com/crewjam/saml/samlsp"
)

var ENROLL_SECRET string

// TODO In memory structures to be removed
// Maps Node Key -> UUID

// Maps Node Key -> Map of Query Name -> Query struct
var queryMap map[string]map[string]Query

// The new DB Wrapper stuff
var db GoServerDatabaseWrapper

func main() {
	ENROLL_SECRET = "somepresharedsecret"
	enableSSO := true
	queryMap = make(map[string]map[string]Query)
	db = GoServerDatabaseWrapper{}
	db.Init()

	// Set up flags for certs
	serverCrt := flag.String("server_cert", "certs/example_server.crt", "Location of a certificate to use")
	serverKey := flag.String("server_key", "certs/example_server.key", "Location of key for certificate")

	ssoCrt := flag.String("sso_cert", "certs/example_goserver_sso.crt", "Location of a certificate to use for sso")
	ssoKey := flag.String("sso_key", "certs/example_goserver_sso.key", "Location of key for certificate for sso")

	flag.Parse()

	// osquery Endpoints
	http.HandleFunc("/enroll", enroll)
	http.HandleFunc("/config", config)
	http.HandleFunc("/log", log)
	http.HandleFunc("/distributedRead", distributedRead)
	http.HandleFunc("/distributedWrite", distributedWrite)

	// goquery Endpoints
	if enableSSO {
		keyPair, err := tls.LoadX509KeyPair(*ssoCrt, *ssoKey)
		if err != nil {
			fmt.Printf("Could not load certificates for SSO\n")
			panic(err)
		}

		keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
		if err != nil {
			panic(err)
		}

		idpMetadataURL, err := url.Parse("http://goserversaml:8002/metadata")
		if err != nil {
			panic(err)
		}

		rootURL, err := url.Parse("https://localhost:8001")
		if err != nil {
			panic(err)
		}

		samlSP, _ := samlsp.New(samlsp.Options{
			URL:               *rootURL,
			Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
			Certificate:       keyPair.Leaf,
			IDPMetadataURL:    idpMetadataURL,
			AllowIDPInitiated: true,
		})

		// Add to service to IDP for some reason happens via client call
		fmt.Printf("Registering ourselves with the IDP Service\n")
		var b bytes.Buffer
		enc := xml.NewEncoder(&b)
		samlSP.ServiceProvider.Metadata().MarshalXML(enc, xml.StartElement{})
		for {
			err = doPut("http://goserversaml:8002/services/:goserver", b.String())
			if err == nil {
				break
			}
		}
		fmt.Printf("Registered ourselves with the IDP Service\n")

		ch := http.HandlerFunc(checkHost)
		sq := http.HandlerFunc(scheduleQuery)
		fr := http.HandlerFunc(fetchResults)

		http.Handle("/checkHost", samlSP.RequireAccount(ch))
		http.Handle("/scheduleQuery", samlSP.RequireAccount(sq))
		http.Handle("/fetchResults", samlSP.RequireAccount(fr))
		http.Handle("/saml/", samlSP)
	} else {
		http.HandleFunc("/checkHost", checkHost)
		http.HandleFunc("/scheduleQuery", scheduleQuery)
		http.HandleFunc("/fetchResults", fetchResults)
	}
	fmt.Printf("Starting test goquery/osquery backend...\n")
	fmt.Printf("Server Cert Path: %s\n", *serverCrt)
	fmt.Printf("Server Key Path:  %s\n", *serverKey)

	err := http.ListenAndServeTLS(":8001", *serverCrt, *serverKey, nil)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
}
