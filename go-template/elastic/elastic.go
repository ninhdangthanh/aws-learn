package elastic

import (
	"log"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

var (
	esInstance *elasticsearch.Client
	esOnce     sync.Once
)

func InitElasticSearch(url string) {
	esOnce.Do(func() {
		cfg := elasticsearch.Config{
			Addresses: []string{url},
		}
		var err error

		for i := 0; i < 5; i++ {
			esInstance, err = elasticsearch.NewClient(cfg)
			if err == nil {
				res, errInfo := esInstance.Info()
				if errInfo == nil {
					res.Body.Close()
					log.Println("Connected to ElasticSearch successfully")
					EnsureIndices()
					return
				}
				err = errInfo
			}
			log.Printf("Waiting for ElasticSearch (attempt %d/5)... error: %v", i+1, err)
			time.Sleep(5 * time.Second)
		}
		log.Fatalf("Could not connect to ElasticSearch after 5 attempts: %v", err)
	})
}

func GetInstance() *elasticsearch.Client {
	if esInstance == nil {
		log.Println("ElasticSearch instance is nil. Did you call InitElasticSearch?")
	}
	return esInstance
}

