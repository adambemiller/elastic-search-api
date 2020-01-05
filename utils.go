package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func indexQuery(es *elasticsearch.Client, i string, q string) error {
	var (
		b strings.Builder
	)

	b.WriteString(`{"query" : "`)
	b.WriteString(q)
	b.WriteString(`"}`)

	qb := []byte(q)
	tb := []byte(time.Now().Format("2006-01-02 15:04:05"))
	idBytes := md5.Sum(append(qb, tb...))
	idHash := hex.EncodeToString(idBytes[:])
	indexReq := esapi.IndexRequest{
		Index:      i + "-queries",
		DocumentID: idHash,
		Body:       strings.NewReader(b.String()),
		Refresh:    "true",
	}

	// Perform the request with the client.
	res, err := indexReq.Do(context.Background(), es)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		err := fmt.Errorf("[%s] Error indexing document ID=%s", res.Status(), idHash)
		return err
	}
	// Deserialize the response into a map.
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		e := fmt.Errorf("Error parsing the index response body: %s", err)
		return e
	}

	// Print the response status and indexed document version.
	err = fmt.Errorf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
	return err
}

func searchQuery(es *elasticsearch.Client, i string, q string) (w http.ResponseWriter, err error) {
	var (
		buf bytes.Buffer
	)

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"title": q,
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	searchRes, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(i),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer searchRes.Body.Close()

	if searchRes.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(searchRes.Body).Decode(&e); err != nil {
			jErr := fmt.Errorf("Error parsing the response body: %s", err)
			return nil, jErr
		}
		// Print the response status and error information.
		err = fmt.Errorf("[%s] %s: %s",
			searchRes.Status(),
			e["error"].(map[string]interface{})["type"],
			e["error"].(map[string]interface{})["reason"],
		)
		return nil, err
	}

	return w, nil
}
