package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Validator validates envelopes against JSON Schema contracts.
type Validator struct {
	envelope    *jsonschema.Schema
	typeSchemas map[string]*jsonschema.Schema
}

// NewValidator loads schemas from the given directory and returns a Validator.
func NewValidator(schemasDir string) (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020

	v := &Validator{
		typeSchemas: make(map[string]*jsonschema.Schema),
	}

	// Collect all .json schema files
	type schemaFile struct {
		rel  string
		path string
	}
	var files []schemaFile
	err := filepath.Walk(schemasDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			rel, _ := filepath.Rel(schemasDir, path)
			files = append(files, schemaFile{rel: filepath.ToSlash(rel), path: path})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk schemas: %w", err)
	}

	// Add all schemas as resources first (for potential $ref resolution)
	for _, sf := range files {
		f, err := os.Open(sf.path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", sf.rel, err)
		}
		if err := c.AddResource(sf.rel, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("add resource %s: %w", sf.rel, err)
		}
		f.Close()
	}

	// Compile envelope schema
	envSch, err := c.Compile("envelope.json")
	if err != nil {
		return nil, fmt.Errorf("compile envelope: %w", err)
	}
	v.envelope = envSch

	// Compile type-specific schemas and map by message type
	for _, sf := range files {
		if sf.rel == "envelope.json" {
			continue
		}
		msgType := extractConstType(sf.path)
		if msgType == "" {
			continue
		}
		sch, err := c.Compile(sf.rel)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", sf.rel, err)
		}
		v.typeSchemas[msgType] = sch
	}

	return v, nil
}

// Validate checks an Envelope against the envelope schema and its type-specific schema.
func (v *Validator) Validate(env Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	return v.ValidateJSON(data)
}

// ValidateJSON validates raw JSON bytes against the envelope schema and the type-specific schema.
func (v *Validator) ValidateJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate against envelope schema
	if err := v.envelope.Validate(value); err != nil {
		return fmt.Errorf("envelope validation failed: %w", err)
	}

	// Extract type for type-specific validation
	var partial struct {
		Type string `json:"type"`
	}
	json.Unmarshal(data, &partial)

	if sch, ok := v.typeSchemas[partial.Type]; ok {
		if err := sch.Validate(value); err != nil {
			return fmt.Errorf("schema validation for %q failed: %w", partial.Type, err)
		}
	}

	return nil
}

// extractConstType reads a schema file and returns the const value of the "type" property, if any.
func extractConstType(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var schema struct {
		Properties struct {
			Type struct {
				Const string `json:"const"`
			} `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		return ""
	}
	return schema.Properties.Type.Const
}
