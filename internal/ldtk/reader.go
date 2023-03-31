package ldtk

import (
	"encoding/json"
	"io"
)

func UnmarshalLdtkReader(r io.Reader) (LdtkJSON, error) {
	var result LdtkJSON
	err := json.NewDecoder(r).Decode(&result)
	return result, err
}
