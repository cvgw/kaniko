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

import (
	"errors"
	"fmt"

	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sirupsen/logrus"
)

type BaseCommand struct {
	cached bool
	cachingCommand
	img v1.Image
}

func (b *BaseCommand) SetCached(cached bool) {
	b.cached = cached
}

func (b *BaseCommand) SetImage(image v1.Image) {
	b.img = image
}

func (b *BaseCommand) setCachedInfo() error {
	logrus.Infof("Found cached layer")
	var err error

	if b.img == nil {
		return errors.New("command image is nil")
	}
	layers, err := b.img.Layers()
	if err != nil {
		return err
	}

	if len(layers) != 1 {
		return errors.New(fmt.Sprintf("expected %d layers but got %d", 1, len(layers)))
	}
	b.layer = layers[0]
	b.readSuccess = true

	return nil
}

func (b *BaseCommand) FilesToSnapshot() []string {
	return []string{}
}

func (b *BaseCommand) FilesUsedFromContext(_ *v1.Config, _ *dockerfile.BuildArgs) ([]string, error) {
	return []string{}, nil
}

func (b *BaseCommand) MetadataOnly() bool {
	return true
}

func (b *BaseCommand) RequiresUnpackedFS() bool {
	return false
}

func (b *BaseCommand) ShouldCacheOutput() bool {
	return true
}
