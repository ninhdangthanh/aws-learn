package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-template/models"
)

func IndexUser(user *models.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	res, err := GetInstance().Index(
		"users",
		bytes.NewReader(data),
		GetInstance().Index.WithDocumentID(fmt.Sprintf("%d", user.ID)),
		GetInstance().Index.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
