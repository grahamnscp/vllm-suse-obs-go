package models

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// variables
var apiBaseURL = fmt.Sprintf("https://%s/v1", os.Getenv("OPENAI_HOSTNAME"))

// var apiKey = os.Getenv("OPENAI_API_KEY")
var aiModel = "empty"
var list ListObject

// Main
func GetAIModel() string {

	//fmt.Printf("GetAIModel: Called\n")

	apiURL := apiBaseURL + "/models"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Fatalf("Error making GET request to %s: %v", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received non-OK HTTP status code: %d", resp.StatusCode)
	}

	modelsData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	// models
	err = yaml.Unmarshal([]byte(modelsData), &list)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for i, item := range list.Data {
		//fmt.Printf("GetAIModel: model[%d]: %s\n", i, item.ID)
		if i == 0 {
			aiModel = item.ID
		}
	}
	//fmt.Printf("GetAIModel: Selected model: %s\n", aiModel)

	return aiModel
}
