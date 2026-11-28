package provider

import (
	"encoding/json"
	"fmt"

	"github.com/swaggest/openapi-go/openapi3"
	"google.golang.org/protobuf/types/known/structpb"
)

// SchemaToStruct encodes an OpenAPI schema as a protobuf Struct so it can be
// sent in a gRPC capability. An empty schema encodes to nil.
func SchemaToStruct(s openapi3.SchemaOrRef) (*structpb.Struct, error) {
	if s.Schema == nil && s.SchemaReference == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal schema: %w", err)
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unable to decode schema: %w", err)
	}
	return structpb.NewStruct(m)
}

// StructToSchema is the inverse of SchemaToStruct. A nil Struct decodes to the
// zero schema.
func StructToSchema(st *structpb.Struct) (openapi3.SchemaOrRef, error) {
	s := openapi3.SchemaOrRef{}
	if st == nil {
		return s, nil
	}
	b, err := json.Marshal(st.AsMap())
	if err != nil {
		return s, fmt.Errorf("unable to marshal schema struct: %w", err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("unable to decode schema: %w", err)
	}
	return s, nil
}
