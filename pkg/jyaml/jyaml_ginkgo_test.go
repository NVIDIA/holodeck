/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package jyaml_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/pkg/jyaml"
)

type SimpleStruct struct {
	Name  string `json:"name" yaml:"name"`
	Value int    `json:"value" yaml:"value"`
}

type NestedStruct struct {
	ID     string       `json:"id" yaml:"id"`
	Nested SimpleStruct `json:"nested" yaml:"nested"`
}

var _ = Describe("JYaml", func() {

	Describe("MarshalJSON", func() {
		It("should marshal simple struct to JSON", func() {
			data := SimpleStruct{Name: "test", Value: 42}
			result, err := jyaml.MarshalJSON(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring(`"name":"test"`))
			Expect(string(result)).To(ContainSubstring(`"value":42`))
		})

		It("should marshal nil value", func() {
			result, err := jyaml.MarshalJSON(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(Equal("null"))
		})

		It("should marshal empty struct", func() {
			data := SimpleStruct{}
			result, err := jyaml.MarshalJSON(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring(`"name":""`))
		})

		It("should marshal nested struct", func() {
			data := NestedStruct{
				ID:     "parent",
				Nested: SimpleStruct{Name: "child", Value: 100},
			}
			result, err := jyaml.MarshalJSON(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring(`"id":"parent"`))
			Expect(string(result)).To(ContainSubstring(`"nested"`))
		})
	})

	Describe("MarshalJSONIndent", func() {
		It("should marshal with proper indentation", func() {
			data := SimpleStruct{Name: "test", Value: 42}
			result, err := jyaml.MarshalJSONIndent(data, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("\n"))
			Expect(string(result)).To(ContainSubstring("  "))
		})

		It("should use custom prefix", func() {
			data := SimpleStruct{Name: "test", Value: 42}
			result, err := jyaml.MarshalJSONIndent(data, ">>", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring(">>"))
		})
	})

	Describe("MarshalYAML", func() {
		It("should marshal simple struct to YAML", func() {
			data := SimpleStruct{Name: "test", Value: 42}
			result, err := jyaml.MarshalYAML(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("name: test"))
			Expect(string(result)).To(ContainSubstring("value: 42"))
		})

		It("should marshal slice", func() {
			data := []SimpleStruct{
				{Name: "first", Value: 1},
				{Name: "second", Value: 2},
			}
			result, err := jyaml.MarshalYAML(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("- name: first"))
		})

		It("should marshal map", func() {
			data := map[string]int{
				"one": 1,
				"two": 2,
			}
			result, err := jyaml.MarshalYAML(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("one: 1"))
		})
	})

	Describe("Unmarshal", func() {
		Context("with string input", func() {
			It("should unmarshal YAML string", func() {
				yamlStr := "name: test\nvalue: 42"
				result, err := jyaml.Unmarshal[SimpleStruct](yamlStr)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal("test"))
				Expect(result.Value).To(Equal(42))
			})
		})

		Context("with []byte input", func() {
			It("should unmarshal YAML bytes", func() {
				yamlBytes := []byte("name: test\nvalue: 42")
				result, err := jyaml.Unmarshal[SimpleStruct](yamlBytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal("test"))
				Expect(result.Value).To(Equal(42))
			})
		})

		Context("with struct input", func() {
			It("should return the same struct", func() {
				input := SimpleStruct{Name: "test", Value: 42}
				result, err := jyaml.Unmarshal[SimpleStruct](input)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(input))
			})
		})

		Context("with map input", func() {
			It("should marshal and unmarshal", func() {
				input := map[string]interface{}{
					"name":  "test",
					"value": 42,
				}
				result, err := jyaml.Unmarshal[SimpleStruct](input)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal("test"))
				Expect(result.Value).To(Equal(42))
			})
		})

		Context("with invalid input", func() {
			It("should return error for type mismatch", func() {
				yamlStr := "name: test\nvalue: not-a-number"
				_, err := jyaml.Unmarshal[SimpleStruct](yamlStr)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UnmarshalStrict", func() {
		Context("with valid input", func() {
			It("should unmarshal without unknown fields", func() {
				yamlStr := "name: test\nvalue: 42"
				result, err := jyaml.UnmarshalStrict[SimpleStruct](yamlStr)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal("test"))
			})
		})

		Context("with unknown fields", func() {
			It("should return error", func() {
				yamlStr := "name: test\nvalue: 42\nunknown: field"
				_, err := jyaml.UnmarshalStrict[SimpleStruct](yamlStr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with struct input", func() {
			It("should return the same struct", func() {
				input := SimpleStruct{Name: "test", Value: 42}
				result, err := jyaml.UnmarshalStrict[SimpleStruct](input)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(input))
			})
		})
	})

	Describe("UnmarshalFromFile", func() {
		var (
			tmpDir  string
			tmpFile string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "jyaml-test-*")
			Expect(err).NotTo(HaveOccurred())
			tmpFile = filepath.Join(tmpDir, "test.yaml")
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		Context("with valid file", func() {
			BeforeEach(func() {
				content := "name: filetest\nvalue: 123"
				err := os.WriteFile(tmpFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should unmarshal file contents", func() {
				result, err := jyaml.UnmarshalFromFile[SimpleStruct](tmpFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal("filetest"))
				Expect(result.Value).To(Equal(123))
			})
		})

		Context("with non-existent file", func() {
			It("should return error", func() {
				_, err := jyaml.UnmarshalFromFile[SimpleStruct](
					"/non/existent/file.yaml")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error reading file"))
			})
		})

		Context("with invalid YAML content", func() {
			BeforeEach(func() {
				content := "name: test\nvalue: not-a-number"
				err := os.WriteFile(tmpFile, []byte(content), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return unmarshal error", func() {
				_, err := jyaml.UnmarshalFromFile[SimpleStruct](tmpFile)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with empty file", func() {
			BeforeEach(func() {
				err := os.WriteFile(tmpFile, []byte(""), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return zero value struct", func() {
				result, err := jyaml.UnmarshalFromFile[SimpleStruct](tmpFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal(""))
				Expect(result.Value).To(Equal(0))
			})
		})
	})
})
