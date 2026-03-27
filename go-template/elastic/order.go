package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-template/models"
)

func IndexOrder(order *models.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	res, err := GetInstance().Index(
		"orders",
		bytes.NewReader(data),
		GetInstance().Index.WithDocumentID(fmt.Sprintf("%d", order.ID)),
		GetInstance().Index.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("error indexing document ID=%d", order.ID)
	}
	return nil
}
