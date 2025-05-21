package jyaml

import (
	"encoding/json"
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

type TestStruct struct {
	Name    string `json:"name" yaml:"name"`
	Age     int    `json:"age" yaml:"age"`
	Address struct {
		Street string `json:"street" yaml:"street"`
		City   string `json:"city" yaml:"city"`
	} `json:"address" yaml:"address"`
}

func TestMarshalJSON(t *testing.T) {
	testData := TestStruct{
		Name: "John",
		Age:  30,
		Address: struct {
			Street string `json:"street" yaml:"street"`
			City   string `json:"city" yaml:"city"`
		}{
			Street: "123 Main St",
			City:   "New York",
		},
	}

	data, err := MarshalJSON(testData)
	if err != nil {
		t.Errorf("MarshalJSON failed: %v", err)
	}

	// Verify the marshaled data can be unmarshaled back
	var result TestStruct
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}

	if result.Name != testData.Name {
		t.Errorf("Expected Name %q, got %q", testData.Name, result.Name)
	}
	if result.Age != testData.Age {
		t.Errorf("Expected Age %d, got %d", testData.Age, result.Age)
	}
}

func TestMarshalJSONIndent(t *testing.T) {
	testData := TestStruct{
		Name: "John",
		Age:  30,
	}

	data, err := MarshalJSONIndent(testData, "", "  ")
	if err != nil {
		t.Errorf("MarshalJSONIndent failed: %v", err)
	}

	// Verify the marshaled data can be unmarshaled back
	var result TestStruct
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}

	if result.Name != testData.Name {
		t.Errorf("Expected Name %q, got %q", testData.Name, result.Name)
	}
}

func TestMarshalYAML(t *testing.T) {
	testData := TestStruct{
		Name: "John",
		Age:  30,
	}

	data, err := MarshalYAML(testData)
	if err != nil {
		t.Errorf("MarshalYAML failed: %v", err)
	}

	// Verify the marshaled data can be unmarshaled back
	var result TestStruct
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Errorf("Failed to unmarshal YAML: %v", err)
	}

	if result.Name != testData.Name {
		t.Errorf("Expected Name %q, got %q", testData.Name, result.Name)
	}
}

func TestUnmarshal(t *testing.T) {
	yamlStr := `
name: John
age: 30
address:
  street: 123 Main St
  city: New York
`

	result, err := Unmarshal[TestStruct](yamlStr)
	if err != nil {
		t.Errorf("Unmarshal failed: %v", err)
	}

	if result.Name != "John" {
		t.Errorf("Expected Name %q, got %q", "John", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("Expected Age %d, got %d", 30, result.Age)
	}
}

func TestUnmarshalStrict(t *testing.T) {
	yamlStr := `
name: John
age: 30
address:
  street: 123 Main St
  city: New York
`

	result, err := UnmarshalStrict[TestStruct](yamlStr)
	if err != nil {
		t.Errorf("UnmarshalStrict failed: %v", err)
	}

	if result.Name != "John" {
		t.Errorf("Expected Name %q, got %q", "John", result.Name)
	}
}

func TestUnmarshalFromFile(t *testing.T) {
	// Create a temporary file with test data
	tempFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name()) //nolint:errcheck,gosec

	yamlContent := `
name: John
age: 30
address:
  street: 123 Main St
  city: New York
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Errorf("failed to close temp file: %v", err)
	}

	result, err := UnmarshalFromFile[TestStruct](tempFile.Name())
	if err != nil {
		t.Errorf("UnmarshalFromFile failed: %v", err)
	}

	if result.Name != "John" {
		t.Errorf("Expected Name %q, got %q", "John", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("Expected Age %d, got %d", 30, result.Age)
	}
}

func TestUnmarshalWithInvalidData(t *testing.T) {
	invalidYAML := `
name: John
age: "not a number"  # This should be a number
`

	_, err := Unmarshal[TestStruct](invalidYAML)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestUnmarshalStrictWithUnknownFields(t *testing.T) {
	yamlWithUnknownFields := `
name: John
age: 30
unknown_field: value
`

	_, err := UnmarshalStrict[TestStruct](yamlWithUnknownFields)
	if err == nil {
		t.Error("Expected error for unknown fields in strict mode, got nil")
	}
}
