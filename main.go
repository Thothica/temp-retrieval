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
	http.Handle("POST /libertarian-chunks", httplog.Logger(http.HandlerFunc(HandleLibertarianChunks)))
	http.Handle("POST /legaltext", httplog.Logger(http.HandlerFunc(HandleLegalText)))
	http.Handle("POST /loc", httplog.Logger(http.HandlerFunc(HandleLoc)))
	http.Handle("POST /indian-lit", httplog.Logger(http.HandlerFunc(HandleIndianLit)))
	http.Handle("POST /openalex", httplog.Logger(http.HandlerFunc(HandleOpenalex)))
	http.Handle("/not_found", httplog.Logger(http.NotFoundHandler()))
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func resultTransformation(data *[]interface{}, sourceTransformation func(source *map[string]interface{})) {
	dataCopy := *data
	for idx, val := range dataCopy {
		valMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		source := valMap["_source"].(map[string]interface{})
		sourceTransformation(&source)
		source["Results_id"] = fmt.Sprintf("%s", valMap["_id"].(string))

		for k, v := range source {
			vstr, ok := v.(string)
			if !ok {
				continue
			}
			vstr += fmt.Sprintf("\t%s", valMap["_id"].(string))
			source[k] = vstr
		}

		valMap["_source"] = source
		dataCopy[idx] = valMap
	}
	data = &dataCopy
}

func SemanitcSearch(body *strings.Reader, index string, sourceTransformation func(source *map[string]interface{})) ([]byte, error) {
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
	resultTransformation(&data, sourceTransformation)

	responseData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return responseData, nil
}

func HandleOpenalex(w http.ResponseWriter, r *http.Request) {
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
                                "Text_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Text_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, req.Query, req.K, req.Size))

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
	}

	res, err := SemanitcSearch(searchBody, "openalex-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func HandleIndianLit(w http.ResponseWriter, r *http.Request) {
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
                                "Raw_response_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Raw_response_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, req.Query, req.K, req.Size))

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf("Author:\t%v\n\nBook:\t%v\n\nChapter:\t%v\n\nEditor:\t%v\n\nInterpretation:\t%v\n\nParagraph:\t%v\n\nPublication:\t%v\n\nSubject:\t%v\n\nTitle:\t%v\n\nTranslation:\t%v\n\nUrl:\t%v\n\nInput_token:\t%v\n\nOutput_token:\t%v",
			source["Author"],
			source["Book"],
			source["Chapter"],
			source["Editor"],
			source["Interpretation"],
			source["Paragraph"],
			source["Publication"],
			source["Subject"],
			source["Title"],
			source["Translation"],
			source["Url"],
			source["Input_token"],
			source["Output_token"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "indic-lit-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func HandleLoc(w http.ResponseWriter, r *http.Request) {
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
                                "Text_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Text_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, req.Query, req.K, req.Size))

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
	}

	res, err := SemanitcSearch(searchBody, "loc-new-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func HandleLegalText(w http.ResponseWriter, r *http.Request) {
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

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf("Title: %v \n Url: %v \n summary: %v \n\n Interesting detail: %v\n%v", source["Title"], source["URL"], source["explanation"], source["answer1"], source["answer2"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "legaltext-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func HandleLibertarianChunks(w http.ResponseWriter, r *http.Request) {
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
                                "Text_embedding"
                        ]
                },
                "query": {
                        "neural": {
                                "Text_embedding": {
                                        "query_text": "%v",
                                        "model_id": "AbDZGo8BB3UUeZ_94CHA",
                                        "k": %v
                                }
                        }
                },
                "size": %v
        }`, req.Query, req.K, req.Size))

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf(`Book title: %v

Author(s):

%v

Date: %v 

Publisher: %v 

Text: 

%v

URL: %v`, source["Title"], source["Author"], source["Date"], source["Publisher"], source["Text"], source["TITLE_URL"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "libertarian-chunks-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
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

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf(`Book title: %v %v

Author(s):

%v

Date: %v 

Publisher: %v 

Translated page content:

%v

URL: %v`, source["Title"], source["Title_Transliterated"], source["Author"], source["Date"], source["Publisher"], source["translation"], source["PDF_URL"])

		source["Results_nonEnglish"] = fmt.Sprintf("Book title: %v \n Author(s): %v, Date: %v, Publisher: %v, Url: %v \n\n content: \n %v", source["Title"], source["Author"],
			source["Date"], source["Publisher"], source["PDF_URL"], source["Text"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "cleaned-arabicbooks-index", sourceTransformation)
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

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf(`Title: %v

Translated Text:
%v

Interpretation:
%v`, source["title"], source["translation"], source["interpretation"])

		source["Results_nonEnglish"] = fmt.Sprintf("Title: %v \n content: %v", source["title"], source["Text"])

		source["Results_orignal"] = fmt.Sprintf("Title: %v \n content: %v", source["title"], source["Text"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "cleaned-dutchtext-index", sourceTransformation)
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

	sourceTransformation := func(sourceOrg *map[string]interface{}) {
		source := *sourceOrg
		source["Results"] = fmt.Sprintf(`Title: %v | Translated: %v
Poet: %v from %v
Translated Text: %v`, source["title"], source["translated_title"], source["Poet"], source["Era"], source["translated_poem"])
		source["Results_nonEnglish"] = fmt.Sprintf("Title: %v \n Poet: %v, \n\n Poem: %v", source["title"], source["Poet"], source["poem"])
		source["Results_orignal"] = fmt.Sprintf("Title: %v, \n Poem: %v", source["title"], source["poem"])
		sourceOrg = &source
	}

	res, err := SemanitcSearch(searchBody, "arabic-poems-index", sourceTransformation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}
