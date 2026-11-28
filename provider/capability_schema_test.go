package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/swaggest/openapi-go/openapi3"
)

type schemaTestInput struct {
	Pattern  string   `json:"pattern"`
	Location string   `json:"location,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
}

type schemaTestOutput struct {
	Matched bool `json:"matched"`
}

type schemaTestClient struct {
	BaseClient
	caps []Capability
}

func (c *schemaTestClient) Capabilities() []Capability { return c.caps }

// sameSchema compares what goes on the wire, since reflected schemas also carry
// an in-memory ReflectType that is never serialized.
func sameSchema(t *testing.T, a, b openapi3.SchemaOrRef) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(ja) == string(jb)
}

func newSchemaTestCap(t *testing.T, name string) Capability {
	t.Helper()
	cap, err := ToProviderInputOutputCap(openapi3.NewReflector(), logr.Discard(), schemaTestInput{}, schemaTestOutput{}, name)
	if err != nil {
		t.Fatalf("unable to build capability: %v", err)
	}
	if cap.Input.Schema == nil || cap.Output.Schema == nil {
		t.Fatalf("expected reflected schemas, got input=%v output=%v", cap.Input.Schema, cap.Output.Schema)
	}
	return cap
}

func TestSchemaRoundTrip(t *testing.T) {
	cap := newSchemaTestCap(t, "referenced")

	for name, in := range map[string]openapi3.SchemaOrRef{"input": cap.Input, "output": cap.Output} {
		t.Run(name, func(t *testing.T) {
			st, err := SchemaToStruct(in)
			if err != nil {
				t.Fatalf("SchemaToStruct: %v", err)
			}
			if st == nil {
				t.Fatal("expected a struct for a populated schema")
			}
			got, err := StructToSchema(st)
			if err != nil {
				t.Fatalf("StructToSchema: %v", err)
			}
			if !sameSchema(t, got, in) {
				t.Errorf("schema changed in round trip\nwant %+v\ngot  %+v", in.Schema, got.Schema)
			}
		})
	}
}

func TestSchemaRoundTripReference(t *testing.T) {
	in := openapi3.SchemaOrRef{SchemaReference: &openapi3.SchemaReference{Ref: "#/components/schemas/thing"}}
	st, err := SchemaToStruct(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := StructToSchema(st)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaReference == nil || got.SchemaReference.Ref != in.SchemaReference.Ref {
		t.Errorf("reference lost in round trip: %+v", got)
	}
}

func TestSchemaEmpty(t *testing.T) {
	st, err := SchemaToStruct(openapi3.SchemaOrRef{})
	if err != nil || st != nil {
		t.Fatalf("empty schema should encode to nil, got %v, %v", st, err)
	}
	got, err := StructToSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != nil || got.SchemaReference != nil {
		t.Errorf("nil struct should decode to the zero schema, got %+v", got)
	}
}

func TestServerCapabilitiesCarriesSchemas(t *testing.T) {
	withSchemas := newSchemaTestCap(t, "referenced")
	s := &server{Client: &schemaTestClient{caps: []Capability{withSchemas, {Name: "dependency"}}}}

	resp, err := s.Capabilities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(resp.Capabilities))
	}

	first := resp.Capabilities[0]
	if first.Name != "referenced" || first.Input == nil || first.Output == nil {
		t.Fatalf("schemas missing from capability: %+v", first)
	}
	in, err := StructToSchema(first.Input)
	if err != nil {
		t.Fatal(err)
	}
	if !sameSchema(t, in, withSchemas.Input) {
		t.Errorf("input schema differs from the provider's")
	}
	if _, ok := in.Schema.Properties["pattern"]; !ok {
		t.Errorf("expected the pattern property in the input schema, got %v", in.Schema.Properties)
	}

	second := resp.Capabilities[1]
	if second.Input != nil || second.Output != nil {
		t.Errorf("capability without schemas should not carry any, got %+v", second)
	}
}
