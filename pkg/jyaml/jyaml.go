/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package jyaml

import (
	"encoding/json"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

func MarshalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func MarshalJSONIndent(v any, prefix, indent string) ([]byte, error) {
	data, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func MarshalYAML(v any) ([]byte, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func Unmarshal[T any](object any) (T, error) {
	var data []byte

	switch v := object.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	case T:
		return v, nil
	default:
		marshaled, err := yaml.Marshal(object)
		if err != nil {
			return *new(T), err
		}
		data = marshaled
	}

	var result T
	if err := yaml.Unmarshal(data, &result); err != nil {
		return *new(T), err
	}

	return result, nil
}

func UnmarshalStrict[T any](object any) (T, error) {
	var data []byte

	switch v := object.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	case T:
		return v, nil
	default:
		marshaled, err := yaml.Marshal(object)
		if err != nil {
			return *new(T), err
		}
		data = marshaled
	}

	var result T
	if err := yaml.UnmarshalStrict(data, &result); err != nil {
		return *new(T), err
	}

	return result, nil
}

func UnmarshalFromFile[T any](filename string) (T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return *new(T), fmt.Errorf("error reading file: %w", err)
	}
	return Unmarshal[T](data)

