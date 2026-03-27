package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-template/models"
)

func IndexProduct(product *models.Product) error {
	data, err := json.Marshal(product)
	if err != nil {
		return err
	}

	res, err := GetInstance().Index(
		"products",
		bytes.NewReader(data),
		GetInstance().Index.WithDocumentID(fmt.Sprintf("%d", product.ID)),
		GetInstance().Index.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
