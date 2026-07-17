package sqlbuild

import "encoding/json"

func jsonMarshalImpl(v any) ([]byte, error) { return json.Marshal(v) }
func jsonUnmarshalImpl(b []byte, v any) error { return json.Unmarshal(b, v) }
