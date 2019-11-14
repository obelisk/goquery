package main

import (
	"fmt"
	"net/http"
	"math/rand"
	"strings"
	"time"
)

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func doPut(url string, metadata string) error {
	client := &http.Client{}
	request, err := http.NewRequest("PUT", url, strings.NewReader(metadata))
	request.ContentLength = int64(len(metadata))
	_, err = client.Do(request)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}
	return nil
}

