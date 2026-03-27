package elastic

import (
	"context"
	"fmt"
	"log"
	"strings"
)

const commonSettings = `{
	"analysis": {
		"filter": {
			"ngram_filter": {
				"type": "ngram",
				"min_gram": 2,
				"max_gram": 10
			}
		},
		"analyzer": {
			"ngram_analyzer": {
				"type": "custom",
				"tokenizer": "standard",
				"filter": [
					"lowercase",
					"ngram_filter"
				]
			}
		}
	}
}`

var indexMappings = map[string]string{
	"users": fmt.Sprintf(`{
		"settings": %s,
		"mappings": {
			"properties": {
				"name": { "type": "text", "analyzer": "ngram_analyzer", "search_analyzer": "standard" },
				"email": { "type": "text", "analyzer": "ngram_analyzer", "search_analyzer": "standard" }
			}
		}
	}`, commonSettings),

	"products": fmt.Sprintf(`{
		"settings": %s,
		"mappings": {
			"properties": {
				"name": { "type": "text", "analyzer": "ngram_analyzer", "search_analyzer": "standard" },
				"sku": { "type": "text", "analyzer": "ngram_analyzer", "search_analyzer": "standard" }
			}
		}
	}`, commonSettings),

	"orders": fmt.Sprintf(`{
		"settings": %s,
		"mappings": {
			"properties": {
				"item_name": { "type": "text", "analyzer": "ngram_analyzer", "search_analyzer": "standard" }
			}
		}
	}`, commonSettings),
}

func EnsureIndices() {
	client := GetInstance()
	if client == nil {
		log.Println("EnsureIndices: Elastic Search instance is nil")
		return
	}
	ctx := context.Background()

	for index, mapping := range indexMappings {
		res, err := client.Indices.Exists([]string{index})
		if err != nil {
			log.Printf("Error checking existence of index %s: %v", index, err)
			continue
		}
		if res.StatusCode == 200 {
			res.Body.Close()
			continue
		}
		res.Body.Close()

		res, err = client.Indices.Create(
			index,
			client.Indices.Create.WithBody(strings.NewReader(mapping)),
			client.Indices.Create.WithContext(ctx),
		)
		if err != nil {
			log.Printf("Error creating index %s: %v", index, err)
			continue
		}
		res.Body.Close()
		log.Printf("Successfully created index: %s with custom ngram mapping", index)
	}
}
