package repository

import "encoding/json"

func derefRawMessage(value *json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}

	return *value
}
