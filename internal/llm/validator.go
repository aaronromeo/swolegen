package llm

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

func ValidateAnalyzerJSON(b []byte) error {
	loader := gojsonschema.NewBytesLoader(b)
	schemaLoader := gojsonschema.NewStringLoader(AnalyzerSchema)
	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}
	if !result.Valid() {
		return fmt.Errorf("analyzer json invalid: %s", collect(result.Errors()))
	}
	return nil
}

func ValidateWorkoutYAML(b []byte) error {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("yaml parse: %w", err)
	}
	jb, err := json.Marshal(v)
	if err != nil {
		return err
	}
	loader := gojsonschema.NewBytesLoader(jb)
	schemaLoader := gojsonschema.NewReferenceLoader(WorkoutSchema)
	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}
	if !result.Valid() {
		return fmt.Errorf("workout yaml invalid: %s", collect(result.Errors()))
	}
	return nil
}

func collect(errs []gojsonschema.ResultError) string {
	var buf bytes.Buffer
	for _, e := range errs {
		buf.WriteString(e.String())
		buf.WriteByte(';')
	}
	return buf.String()
}
