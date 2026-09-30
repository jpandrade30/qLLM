package sqlbuild

import "encoding/json"

// jsonMarshalImpl implements runtime behavior for this package.
func jsonMarshalImpl(v any) ([]byte, error) { return json.Marshal(v) }

// jsonUnmarshalImpl implements runtime behavior for this package.
func jsonUnmarshalImpl(b []byte, v any) error { return json.Unmarshal(b, v) }
