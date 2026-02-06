/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// mockTableData implements TableData for testing
type mockTableData struct {
	headers []string
	rows    [][]string
}

func (m *mockTableData) Headers() []string { return m.headers }
func (m *mockTableData) Rows() [][]string  { return m.rows }

// testStruct is a sample struct for JSON/YAML serialization tests
type testStruct struct {
	Name   string `json:"name" yaml:"name"`
	Value  int    `json:"value" yaml:"value"`
	Active bool   `json:"active" yaml:"active"`
}

func TestValidFormats(t *testing.T) {
	formats := ValidFormats()

	if len(formats) != 3 {
		t.Errorf("expected 3 formats, got %d", len(formats))
	}

	expected := map[string]bool{
		"table": false,
		"json":  false,
		"yaml":  false,
	}

	for _, f := range formats {
		if _, ok := expected[f]; !ok {
			t.Errorf("unexpected format: %s", f)
		}
		expected[f] = true
	}

	for f, found := range expected {
		if !found {
			t.Errorf("expected format %q not found in ValidFormats()", f)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected bool
	}{
		{
			name:     "table format is valid",
			format:   "table",
			expected: true,
		},
		{
			name:     "json format is valid",
			format:   "json",
			expected: true,
		},
		{
			name:     "yaml format is valid",
			format:   "yaml",
			expected: true,
		},
		{
			name:     "empty string is invalid",
			format:   "",
			expected: false,
		},
		{
			name:     "unknown format is invalid",
			format:   "xml",
			expected: false,
		},
		{
			name:     "uppercase TABLE is invalid",
			format:   "TABLE",
			expected: false,
		},
		{
			name:     "mixed case Json is invalid",
			format:   "Json",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidFormat(tt.format)
			if result != tt.expected {
				t.Errorf("IsValidFormat(%q) = %v, expected %v", tt.format, result, tt.expected)
			}
		})
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		expectedFormat Format
		expectError    bool
	}{
		{
			name:           "empty format defaults to table",
			format:         "",
			expectedFormat: FormatTable,
			expectError:    false,
		},
		{
			name:           "table format creates formatter",
			format:         "table",
			expectedFormat: FormatTable,
			expectError:    false,
		},
		{
			name:           "json format creates formatter",
			format:         "json",
			expectedFormat: FormatJSON,
			expectError:    false,
		},
		{
			name:           "yaml format creates formatter",
			format:         "yaml",
			expectedFormat: FormatYAML,
			expectError:    false,
		},
		{
			name:        "invalid format returns error",
			format:      "xml",
			expectError: true,
		},
		{
			name:        "uppercase format returns error",
			format:      "JSON",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter(tt.format)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for format %q, got nil", tt.format)
				}
				if formatter != nil {
					t.Errorf("expected nil formatter when error occurs")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if formatter == nil {
				t.Error("expected non-nil formatter")
				return
			}

			if formatter.Format() != tt.expectedFormat {
				t.Errorf("formatter.Format() = %v, expected %v", formatter.Format(), tt.expectedFormat)
			}
		})
	}
}

func TestFormatterFormat(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		expectedFormat Format
	}{
		{
			name:           "returns table format",
			format:         "table",
			expectedFormat: FormatTable,
		},
		{
			name:           "returns json format",
			format:         "json",
			expectedFormat: FormatJSON,
		},
		{
			name:           "returns yaml format",
			format:         "yaml",
			expectedFormat: FormatYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter(tt.format)
			if err != nil {
				t.Fatalf("unexpected error creating formatter: %v", err)
			}

			if formatter.Format() != tt.expectedFormat {
				t.Errorf("Format() = %v, expected %v", formatter.Format(), tt.expectedFormat)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		validate func(t *testing.T, output string)
	}{
		{
			name: "struct outputs valid JSON with indentation",
			data: testStruct{Name: "test", Value: 42, Active: true},
			validate: func(t *testing.T, output string) {
				// Verify it's valid JSON
				var result testStruct
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid JSON: %v", err)
				}
				// Verify values
				if result.Name != "test" {
					t.Errorf("expected Name=test, got %s", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("expected Value=42, got %d", result.Value)
				}
				if !result.Active {
					t.Error("expected Active=true")
				}
				// Verify indentation (should have newlines due to SetIndent)
				if !strings.Contains(output, "\n") {
					t.Error("expected indented JSON with newlines")
				}
			},
		},
		{
			name: "map outputs valid JSON",
			data: map[string]interface{}{"key": "value", "number": 123},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid JSON: %v", err)
				}
				if result["key"] != "value" {
					t.Errorf("expected key=value, got %v", result["key"])
				}
			},
		},
		{
			name: "slice outputs valid JSON",
			data: []string{"one", "two", "three"},
			validate: func(t *testing.T, output string) {
				var result []string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid JSON: %v", err)
				}
				if len(result) != 3 {
					t.Errorf("expected 3 items, got %d", len(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter("json")
			if err != nil {
				t.Fatalf("failed to create formatter: %v", err)
			}

			var buf bytes.Buffer
			formatter.SetWriter(&buf)

			if err := formatter.PrintJSON(tt.data); err != nil {
				t.Fatalf("PrintJSON failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}

func TestPrintYAML(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		validate func(t *testing.T, output string)
	}{
		{
			name: "struct outputs valid YAML",
			data: testStruct{Name: "test", Value: 42, Active: true},
			validate: func(t *testing.T, output string) {
				// Verify it's valid YAML
				var result testStruct
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid YAML: %v", err)
				}
				// Verify values
				if result.Name != "test" {
					t.Errorf("expected Name=test, got %s", result.Name)
				}
				if result.Value != 42 {
					t.Errorf("expected Value=42, got %d", result.Value)
				}
				if !result.Active {
					t.Error("expected Active=true")
				}
			},
		},
		{
			name: "map outputs valid YAML",
			data: map[string]interface{}{"key": "value", "number": 123},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid YAML: %v", err)
				}
				if result["key"] != "value" {
					t.Errorf("expected key=value, got %v", result["key"])
				}
			},
		},
		{
			name: "nested struct outputs valid YAML",
			data: struct {
				Name  string `yaml:"name"`
				Items []struct {
					ID   int    `yaml:"id"`
					Name string `yaml:"name"`
				} `yaml:"items"`
			}{
				Name: "parent",
				Items: []struct {
					ID   int    `yaml:"id"`
					Name string `yaml:"name"`
				}{
					{ID: 1, Name: "item1"},
					{ID: 2, Name: "item2"},
				},
			},
			validate: func(t *testing.T, output string) {
				// Just verify it parses as valid YAML
				var result map[string]interface{}
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("output is not valid YAML: %v", err)
				}
				if result["name"] != "parent" {
					t.Errorf("expected name=parent, got %v", result["name"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter("yaml")
			if err != nil {
				t.Fatalf("failed to create formatter: %v", err)
			}

			var buf bytes.Buffer
			formatter.SetWriter(&buf)

			if err := formatter.PrintYAML(tt.data); err != nil {
				t.Fatalf("PrintYAML failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}

func TestPrint(t *testing.T) {
	t.Run("json format calls PrintJSON", func(t *testing.T) {
		formatter, err := NewFormatter("json")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := testStruct{Name: "test", Value: 1, Active: true}
		if err := formatter.Print(data); err != nil {
			t.Fatalf("Print failed: %v", err)
		}

		// Verify output is JSON
		var result testStruct
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("expected JSON output, got: %s", buf.String())
		}
	})

	t.Run("yaml format calls PrintYAML", func(t *testing.T) {
		formatter, err := NewFormatter("yaml")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := testStruct{Name: "test", Value: 1, Active: true}
		if err := formatter.Print(data); err != nil {
			t.Fatalf("Print failed: %v", err)
		}

		// Verify output is YAML (contains key: value format)
		if !strings.Contains(buf.String(), "name: test") {
			t.Errorf("expected YAML output, got: %s", buf.String())
		}
	})

	t.Run("table format with TableData calls PrintTable", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := &mockTableData{
			headers: []string{"NAME", "VALUE"},
			rows: [][]string{
				{"item1", "100"},
				{"item2", "200"},
			},
		}

		if err := formatter.Print(data); err != nil {
			t.Fatalf("Print failed: %v", err)
		}

		output := buf.String()
		// Verify table output contains headers and data
		if !strings.Contains(output, "NAME") {
			t.Errorf("expected table output to contain header NAME, got: %s", output)
		}
		if !strings.Contains(output, "item1") {
			t.Errorf("expected table output to contain item1, got: %s", output)
		}
	})

	t.Run("table format without TableData falls back to JSON", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		// Use a regular struct that doesn't implement TableData
		data := testStruct{Name: "test", Value: 42, Active: true}
		if err := formatter.Print(data); err != nil {
			t.Fatalf("Print failed: %v", err)
		}

		// Verify output is JSON (fallback behavior)
		var result testStruct
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("expected JSON fallback output, got: %s", buf.String())
		}
	})
}

func TestPrintTable(t *testing.T) {
	t.Run("outputs header and rows", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := &mockTableData{
			headers: []string{"NAME", "STATUS", "AGE"},
			rows: [][]string{
				{"env-1", "Running", "5m"},
				{"env-2", "Pending", "2m"},
				{"env-3", "Failed", "10m"},
			},
		}

		if err := formatter.PrintTable(data); err != nil {
			t.Fatalf("PrintTable failed: %v", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// Verify we have header + 3 data rows
		if len(lines) != 4 {
			t.Errorf("expected 4 lines (1 header + 3 rows), got %d: %s", len(lines), output)
		}

		// Verify header contains all column names
		header := lines[0]
		for _, h := range data.headers {
			if !strings.Contains(header, h) {
				t.Errorf("header should contain %q, got: %s", h, header)
			}
		}

		// Verify each row contains expected data
		for i, row := range data.rows {
			line := lines[i+1]
			for _, cell := range row {
				if !strings.Contains(line, cell) {
					t.Errorf("row %d should contain %q, got: %s", i, cell, line)
				}
			}
		}
	})

	t.Run("handles empty rows", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := &mockTableData{
			headers: []string{"NAME", "VALUE"},
			rows:    [][]string{},
		}

		if err := formatter.PrintTable(data); err != nil {
			t.Fatalf("PrintTable failed: %v", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// Should only have header line
		if len(lines) != 1 {
			t.Errorf("expected 1 line (header only), got %d: %s", len(lines), output)
		}
	})

	t.Run("handles single column", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		data := &mockTableData{
			headers: []string{"NAME"},
			rows: [][]string{
				{"item1"},
				{"item2"},
			},
		}

		if err := formatter.PrintTable(data); err != nil {
			t.Fatalf("PrintTable failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "NAME") {
			t.Errorf("expected header NAME, got: %s", output)
		}
		if !strings.Contains(output, "item1") {
			t.Errorf("expected item1, got: %s", output)
		}
	})
}

func TestTablePrinter(t *testing.T) {
	t.Run("fluent interface builds table correctly", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		tp := formatter.NewTablePrinter()
		tp.Header("COL1", "COL2", "COL3").
			Row("a", "b", "c").
			Row("d", "e", "f")

		if err := tp.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "COL1") {
			t.Errorf("expected COL1 in output: %s", output)
		}
		if !strings.Contains(output, "a") {
			t.Errorf("expected 'a' in output: %s", output)
		}
	})

	t.Run("Row accepts interface values", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		tp := formatter.NewTablePrinter()
		tp.Header("STRING", "INT", "BOOL").
			Row("text", 42, true)

		if err := tp.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "text") {
			t.Errorf("expected 'text' in output: %s", output)
		}
		if !strings.Contains(output, "42") {
			t.Errorf("expected '42' in output: %s", output)
		}
		if !strings.Contains(output, "true") {
			t.Errorf("expected 'true' in output: %s", output)
		}
	})

	t.Run("multiple flushes work correctly", func(t *testing.T) {
		formatter, err := NewFormatter("table")
		if err != nil {
			t.Fatalf("failed to create formatter: %v", err)
		}

		var buf bytes.Buffer
		formatter.SetWriter(&buf)

		tp := formatter.NewTablePrinter()
		tp.Header("NAME")
		if err := tp.Flush(); err != nil {
			t.Fatalf("first Flush failed: %v", err)
		}

		// Second flush should also succeed
		if err := tp.Flush(); err != nil {
			t.Fatalf("second Flush failed: %v", err)
		}
	})
}

func TestSetWriter(t *testing.T) {
	formatter, err := NewFormatter("json")
	if err != nil {
		t.Fatalf("failed to create formatter: %v", err)
	}

	var buf1, buf2 bytes.Buffer

	// Write to first buffer
	formatter.SetWriter(&buf1)
	if err := formatter.PrintJSON(map[string]string{"key": "value1"}); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	// Write to second buffer
	formatter.SetWriter(&buf2)
	if err := formatter.PrintJSON(map[string]string{"key": "value2"}); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	// Verify both buffers have their expected content
	if !strings.Contains(buf1.String(), "value1") {
		t.Errorf("buf1 should contain value1: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "value2") {
		t.Errorf("buf2 should contain value2: %s", buf2.String())
	}
	if strings.Contains(buf1.String(), "value2") {
		t.Error("buf1 should not contain value2")
	}
}

func TestNewFormatterErrorMessage(t *testing.T) {
	_, err := NewFormatter("invalid")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}

	// Verify error message contains helpful information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid") {
		t.Errorf("error should mention the invalid format: %s", errMsg)
	}
	if !strings.Contains(errMsg, "table") || !strings.Contains(errMsg, "json") || !strings.Contains(errMsg, "yaml") {
		t.Errorf("error should list valid formats: %s", errMsg)
	}
}
