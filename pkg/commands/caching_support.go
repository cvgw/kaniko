/*
Copyright 2018 Google LLC

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

package commands

import v1 "github.com/google/go-containerregistry/pkg/v1"

type CachedCommand interface {
	Layer() v1.Layer
	ReadSuccess() bool
}

type cachingCommand struct {
	layer       v1.Layer
	readSuccess bool
}

func (c cachingCommand) Layer() v1.Layer {
	return c.layer
}

func (c cachingCommand) ReadSuccess() bool {
	return c.readSuccess
}
