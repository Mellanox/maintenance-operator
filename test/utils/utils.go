/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// LoadObjectFromFile loads an object from "objects" directory with given file name
func LoadObjectFromFile(file string) (*unstructured.Unstructured, error) {
	fullPath := filepath.Join("objects", file)
	obj := &unstructured.Unstructured{}
	b, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	b, err = yaml.YAMLToJSON(b)
	if err != nil {
		return nil, err
	}
	if err = obj.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return obj, nil
}

// ToConcrete converts an unstructured object to a concrete object
func ToConcrete[T client.Object](obj *unstructured.Unstructured, concrete T) error {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, concrete)
	if err != nil {
		return fmt.Errorf("error converting unstructured to %T: %v", concrete, err)
	}
	return nil
}
