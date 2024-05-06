package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
)

type RequestBody struct {
	Query string `json:"query"`
	Size  int    `json:"size"`
	K     int    `json:"k"`
}

var (
	ENDPOINT = "http://localhost:9200"
	USER     = "admin"
	PASSWORD = "admin"
	c        *opensearch.Client
)

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Could not read env file")
	}

	if val, ok := os.LookupEnv("ENDPOINT"); ok {
		ENDPOINT = val
	}

	if val, ok := os.LookupEnv("OUSER"); ok {
		USER = val
	}

	if val, ok := os.LookupEnv("PASSWORD"); ok {
		PASSWORD = val
	}

	opensearchConfig := &opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Addresses: []string{ENDPOINT},
		Username:  USER,
		Password:  PASSWORD,
	}

	c, err = opensearch.NewClient(*opensearchConfig)
	if err != nil {
		log.Fatal(err)
	}
	res, err := c.Ping()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)

}

func main() {
	http.HandleFunc("POST /arabic-poems", HandleArabicPoems)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func HandleArabicPoems(w http.ResponseWriter, r *http.Request) {
	var arabicpoemsRequest RequestBody

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&arabicpoemsRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	searchBody := strings.NewReader(fmt.Sprintf(`{
                "_source": {
                        "excludes": [
                                "interpretation_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "interpretation_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, arabicpoemsRequest.Query, arabicpoemsRequest.K, arabicpoemsRequest.Size))

	semanticSearchRequest := opensearchapi.SearchRequest{
		Index: []string{"arabic-poems-index"},
		Body:  searchBody,
	}

	var searchResponse map[string]interface{}

	res, err := semanticSearchRequest.Do(context.Background(), c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&searchResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	data := searchResponse["hits"].(map[string]interface{})["hits"].([]interface{})

	for idx, val := range data {
		valMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		source := valMap["_source"].(map[string]interface{})

		source["Results"] = fmt.Sprintf(`Title: %v | Translated: %v
Poet: %v from %v
Translated Text:
%v`, source["title"], source["translated_title"], source["Poet"], source["Era"], source["translated_poem"])
        
        valMap["_source"] = source
        data[idx] = valMap
	}

	responseData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}
