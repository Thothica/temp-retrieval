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

	"github.com/MadAppGang/httplog"
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
	http.Handle("POST /arabic-poems", httplog.Logger(http.HandlerFunc(HandleArabicPoems)))
	http.Handle("POST /cleaned-dutchtext", httplog.Logger(http.HandlerFunc(HandleCleanedDutchText)))
	http.Handle("POST /cleaned-arabicbooks", httplog.Logger(http.HandlerFunc(HandleCleanedArabicBooks)))
	http.Handle("/not_found", httplog.Logger(http.NotFoundHandler()))
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func SemanitcSearch(body *strings.Reader, index string, transformation func(d *[]interface{})) ([]byte, error) {
	var searchResponse map[string]interface{}

	semanticSearchRequest := opensearchapi.SearchRequest{
		Index: []string{index},
		Body:  body,
	}

	res, err := semanticSearchRequest.Do(context.Background(), c)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&searchResponse)
	if err != nil {
		return nil, err
	}

	data := searchResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	transformation(&data)

	responseData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return responseData, nil
}

func HandleCleanedArabicBooks(w http.ResponseWriter, r *http.Request) {
	var req RequestBody

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	searchBody := strings.NewReader(fmt.Sprintf(`{
                "_source": {
                        "excludes": [
                                "Raw_Response_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Raw_Response_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, req.Query, req.K, req.Size))

	t := func(data *[]interface{}) {
		dataCopy := *data
		for idx, val := range dataCopy {
			valMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}

			source := valMap["_source"].(map[string]interface{})

			source["Results"] = fmt.Sprintf(`Book title: %v %v

Author(s):

%v

Date: %v 

Publisher: %v 

Translated page content:

%v

URL: %v`, source["Title"], source["Title_Transliterated"], source["Author"], source["Date"], source["Publisher"], source["translation"], source["PDF_URL"])

			source["Results_nonEnglish"] = fmt.Sprintf("%v", source["Full_Text"])

			valMap["_source"] = source
			dataCopy[idx] = valMap
		}
		data = &dataCopy
	}

	res, err := SemanitcSearch(searchBody, "cleaned-arabicbooks-index", t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func HandleCleanedDutchText(w http.ResponseWriter, r *http.Request) {
	var dutchtextrequest RequestBody

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&dutchtextrequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	searchBody := strings.NewReader(fmt.Sprintf(`{
                "_source": {
                        "excludes": [
                                "Raw_Response_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Raw_Response_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, dutchtextrequest.Query, dutchtextrequest.K, dutchtextrequest.Size))

	t := func(d *[]interface{}) {
		dataCopy := *d
		for idx, val := range dataCopy {
			valMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}

			source := valMap["_source"].(map[string]interface{})

			source["Results"] = fmt.Sprintf(`Title: %v

Translated Text:
%v

Interpretation:
%v`, source["title"], source["translation"], source["interpretation"])

			source["Results_nonEnglish"] = fmt.Sprintf("%v", source["Text"])

			valMap["_source"] = source
			dataCopy[idx] = valMap
		}
		d = &dataCopy
	}

	res, err := SemanitcSearch(searchBody, "cleaned-dutchtext-index", t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
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

	t := func(d *[]interface{}) {
		dataCopy := *d
		for idx, val := range dataCopy {
			valMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}

			source := valMap["_source"].(map[string]interface{})

			source["Results"] = fmt.Sprintf(`Title: %v | Translated: %v
Poet: %v from %v
Translated Text: %v`, source["title"], source["translated_title"], source["Poet"], source["Era"], source["translated_poem"])

			source["Results_nonEnglish"] = fmt.Sprintf("%v", source["poem"])

			valMap["_source"] = source
			dataCopy[idx] = valMap
		}
		d = &dataCopy
	}

	res, err := SemanitcSearch(searchBody, "arabic-poems-index", t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}
