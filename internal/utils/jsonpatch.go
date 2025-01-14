/*
Copyright 2020 The Kubernetes Authors.

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
	"path"
	"strings"
)

// JSONPatch is a json marshaling helper used for patching API objects
type JSONPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// NewJSONPatch returns a new JsonPatch object
func NewJSONPatch(verb string, jsonpath string, key string, value interface{}) JSONPatch {
	// Note(adrianc): if verb is "remove" then value should be nil
	return JSONPatch{
		Op:    verb,
		Path:  path.Join(jsonpath, strings.ReplaceAll(key, "/", "~1")),
		Value: value,
	}
}
