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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format represents the output format type
type Format string

const (
	// FormatTable outputs data as a formatted table (default)
	FormatTable Format = "table"
	// FormatJSON outputs data as JSON
	FormatJSON Format = "json"
	// FormatYAML outputs data as YAML
	FormatYAML Format = "yaml"
)

// ValidFormats returns all valid output formats
func ValidFormats() []string {
	return []string{string(FormatTable), string(FormatJSON), string(FormatYAML)}
}

// IsValidFormat checks if the given format string is valid
func IsValidFormat(format string) bool {
	switch Format(format) {
	case FormatTable, FormatJSON, FormatYAML:
		return true
	default:
		return false
	}
}

// Formatter handles output formatting for CLI commands
type Formatter struct {
	format Format
	writer io.Writer
}

// NewFormatter creates a new formatter with the specified format
func NewFormatter(format string) (*Formatter, error) {
	if format == "" {
		format = string(FormatTable)
	}
	if !IsValidFormat(format) {
		return nil, fmt.Errorf("invalid output format %q, must be one of: %v", format, ValidFormats())
	}
	return &Formatter{
		format: Format(format),
		writer: os.Stdout,
	}, nil
}

// SetWriter sets the output writer (useful for testing)
func (f *Formatter) SetWriter(w io.Writer) {
	f.writer = w
}

// Format returns the current format
func (f *Formatter) Format() Format {
	return f.format
}

// PrintJSON outputs data as JSON
func (f *Formatter) PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintYAML outputs data as YAML
func (f *Formatter) PrintYAML(data interface{}) error {
	encoder := yaml.NewEncoder(f.writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}

// TablePrinter provides a fluent interface for building tables
type TablePrinter struct {
	writer    *tabwriter.Writer
	formatter *Formatter
}

// NewTablePrinter creates a new table printer
func (f *Formatter) NewTablePrinter() *TablePrinter {
	return &TablePrinter{
		writer:    tabwriter.NewWriter(f.writer, 0, 0, 3, ' ', 0),
		formatter: f,
	}
}

// Header writes the table header
func (t *TablePrinter) Header(columns ...string) *TablePrinter {
	for i, col := range columns {
		if i > 0 {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprint(t.writer, col)
	}
	fmt.Fprintln(t.writer)
	return t
}

// Row writes a table row
func (t *TablePrinter) Row(values ...interface{}) *TablePrinter {
	for i, val := range values {
		if i > 0 {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprint(t.writer, val)
	}
	fmt.Fprintln(t.writer)
	return t
}

// Flush writes the buffered output
func (t *TablePrinter) Flush() error {
	return t.writer.Flush()
}

// Print outputs data in the configured format
// For table format, data should implement TableData interface
// For JSON/YAML, data is serialized directly
func (f *Formatter) Print(data interface{}) error {
	switch f.format {
	case FormatJSON:
		return f.PrintJSON(data)
	case FormatYAML:
		return f.PrintYAML(data)
	case FormatTable:
		// For table format, the caller should use PrintTable or NewTablePrinter
		// This is a fallback that prints as JSON if table isn't implemented
		if td, ok := data.(TableData); ok {
			return f.PrintTable(td)
		}
		// Fallback to JSON for complex types without table implementation
		return f.PrintJSON(data)
	default:
		return fmt.Errorf("unsupported format: %s", f.format)
	}
}

// TableData interface for types that can be rendered as tables
type TableData interface {
	// Headers returns the column headers for the table
	Headers() []string
	// Rows returns the data rows for the table
	Rows() [][]string
}

// PrintTable outputs TableData as a formatted table
func (f *Formatter) PrintTable(data TableData) error {
	tp := f.NewTablePrinter()
	tp.Header(data.Headers()...)
	for _, row := range data.Rows() {
		values := make([]interface{}, len(row))
		for i, v := range row {
			values[i] = v
		}
		tp.Row(values...)
	}
	return tp.Flush()
}
