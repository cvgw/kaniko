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

// for use in tests
package executor

import (
	"errors"

	"github.com/GoogleContainerTools/kaniko/pkg/commands"
	"github.com/GoogleContainerTools/kaniko/pkg/config"
	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func fakeCachePush(_ *config.KanikoOptions, _, _, _ string) error {
	return nil
}

type fakeSnapShotter struct {
	file string
}

func (f fakeSnapShotter) Init() error { return nil }
func (f fakeSnapShotter) TakeSnapshotFS() (string, error) {
	return f.file, nil
}
func (f fakeSnapShotter) TakeSnapshot(_ []string) (string, error) {
	return f.file, nil
}

type MockDockerCommand struct {
	contextFiles []string
	cacheCommand commands.DockerCommand
}

func (m MockDockerCommand) ExecuteCommand(c *v1.Config, args *dockerfile.BuildArgs) error { return nil }
func (m MockDockerCommand) String() string {
	return "meow"
}
func (m MockDockerCommand) FilesToSnapshot() []string {
	return []string{"meow-snapshot-no-cache"}
}
func (m MockDockerCommand) FilesUsedFromContext(c *v1.Config, args *dockerfile.BuildArgs) ([]string, error) {
	return m.contextFiles, nil
}
func (m MockDockerCommand) MetadataOnly() bool {
	return false
}
func (m MockDockerCommand) RequiresUnpackedFS() bool {
	return false
}
func (m MockDockerCommand) ShouldCacheOutput() bool {
	return true
}
func (m MockDockerCommand) SetCached(_ bool)    {}
func (m MockDockerCommand) SetImage(_ v1.Image) {}

type MockCachedDockerCommand struct {
	contextFiles []string
}

func (m MockCachedDockerCommand) ExecuteCommand(c *v1.Config, args *dockerfile.BuildArgs) error {
	return nil
}
func (m MockCachedDockerCommand) String() string {
	return "meow"
}
func (m MockCachedDockerCommand) FilesToSnapshot() []string {
	return []string{"meow-snapshot"}
}
func (m MockCachedDockerCommand) FilesUsedFromContext(c *v1.Config, args *dockerfile.BuildArgs) ([]string, error) {
	return m.contextFiles, nil
}
func (m MockCachedDockerCommand) MetadataOnly() bool {
	return false
}
func (m MockCachedDockerCommand) RequiresUnpackedFS() bool {
	return false
}
func (m MockCachedDockerCommand) ShouldCacheOutput() bool {
	return true
}
func (m MockCachedDockerCommand) SetCached(_ bool)    {}
func (m MockCachedDockerCommand) SetImage(_ v1.Image) {}

type fakeLayerCache struct {
	retrieve bool
}

func (f fakeLayerCache) RetrieveLayer(_ string) (v1.Image, error) {
	if !f.retrieve {
		return nil, errors.New("could not find layer")
	}
	return nil, nil
}
