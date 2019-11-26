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
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	"github.com/GoogleContainerTools/kaniko/testutil"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/sirupsen/logrus"
)

var copyTests = []struct {
	name           string
	sourcesAndDest []string
	expectedDest   []string
}{
	{
		name:           "copy foo into tempCopyExecuteTest/",
		sourcesAndDest: []string{"foo", "tempCopyExecuteTest/"},
		expectedDest:   []string{"foo"},
	},
	{
		name:           "copy foo into tempCopyExecuteTest",
		sourcesAndDest: []string{"foo", "tempCopyExecuteTest"},
		expectedDest:   []string{"tempCopyExecuteTest"},
	},
}

func setupTestTemp() string {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		logrus.Fatalf("error creating temp dir %s", err)
	}
	logrus.Debugf("Tempdir: %s", tempDir)

	srcPath, err := filepath.Abs("../../integration/context")
	if err != nil {
		logrus.Fatalf("error getting abs path %s", srcPath)
	}
	cperr := filepath.Walk(srcPath,
		func(path string, info os.FileInfo, err error) error {
			if path != srcPath {
				if err != nil {
					return err
				}
				tempPath := strings.TrimPrefix(path, srcPath)
				fileInfo, err := os.Stat(path)
				if err != nil {
					return err
				}
				if fileInfo.IsDir() {
					os.MkdirAll(tempDir+"/"+tempPath, 0777)
				} else {
					out, err := os.Create(tempDir + "/" + tempPath)
					if err != nil {
						return err
					}
					defer out.Close()

					in, err := os.Open(path)
					if err != nil {
						return err
					}
					defer in.Close()

					_, err = io.Copy(out, in)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
	if cperr != nil {
		logrus.Fatalf("error populating temp dir %s", cperr)
	}

	return tempDir
}
func TestCopyExecuteCmd(t *testing.T) {
	tempDir := setupTestTemp()
	defer os.RemoveAll(tempDir)

	cfg := &v1.Config{
		Cmd:        nil,
		Env:        []string{},
		WorkingDir: tempDir,
	}

	for _, test := range copyTests {
		t.Run(test.name, func(t *testing.T) {
			dirList := []string{}

			cmd := CopyCommand{
				cmd: &instructions.CopyCommand{
					SourcesAndDest: test.sourcesAndDest,
				},
				buildcontext: tempDir,
			}

			buildArgs := copySetUpBuildArgs()
			dest := cfg.WorkingDir + "/" + test.sourcesAndDest[len(test.sourcesAndDest)-1]

			err := cmd.ExecuteCommand(cfg, buildArgs)
			if err != nil {
				t.Error()
			}

			fi, err := os.Open(dest)
			if err != nil {
				t.Error()
			}
			defer fi.Close()
			fstat, err := fi.Stat()
			if err != nil {
				t.Error()
			}
			if fstat.IsDir() {
				files, err := ioutil.ReadDir(dest)
				if err != nil {
					t.Error()
				}
				for _, file := range files {
					logrus.Debugf("file: %v", file.Name())
					dirList = append(dirList, file.Name())
				}
			} else {
				dirList = append(dirList, filepath.Base(dest))
			}

			testutil.CheckErrorAndDeepEqual(t, false, err, test.expectedDest, dirList)
			os.RemoveAll(dest)
		})
	}
}

func copySetUpBuildArgs() *dockerfile.BuildArgs {
	buildArgs := dockerfile.NewBuildArgs([]string{
		"buildArg1=foo",
		"buildArg2=foo2",
	})
	buildArgs.AddArg("buildArg1", nil)
	d := "default"
	buildArgs.AddArg("buildArg2", &d)
	return buildArgs
}

func Test_resolveIfSymlink(t *testing.T) {
	type testCase struct {
		destPath     string
		expectedPath string
		err          error
	}

	tmpDir, err := ioutil.TempDir("", "copy-test")
	if err != nil {
		t.Error(err)
	}

	baseDir, err := ioutil.TempDir(tmpDir, "not-linked")
	if err != nil {
		t.Error(err)
	}

	path, err := ioutil.TempFile(baseDir, "foo.txt")
	if err != nil {
		t.Error(err)
	}

	thepath, err := filepath.Abs(filepath.Dir(path.Name()))
	if err != nil {
		t.Error(err)
	}
	cases := []testCase{{destPath: thepath, expectedPath: thepath, err: nil}}

	baseDir = tmpDir
	symLink := filepath.Join(baseDir, "symlink")
	if err := os.Symlink(filepath.Base(thepath), symLink); err != nil {
		t.Error(err)
	}
	cases = append(cases, testCase{filepath.Join(symLink, "foo.txt"), filepath.Join(thepath, "foo.txt"), nil})

	for i, c := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			res, e := resolveIfSymlink(c.destPath)
			if e != c.err {
				t.Errorf("%s: expected %v but got %v", c.destPath, c.err, e)
			}

			if res != c.expectedPath {
				t.Errorf("%s: expected %v but got %v", c.destPath, c.expectedPath, res)
			}
		})
	}
}

func Test_CopyCommand_ExecuteCommand(t *testing.T) {
	tarContent, err := prepareTarFixture()
	if err != nil {
		t.Errorf("couldn't prepare tar fixture %v", err)
	}

	config := &v1.Config{}
	buildArgs := &dockerfile.BuildArgs{}

	type testCase struct {
		desctiption string
		expectLayer bool
		expectErr   bool
		count       *int
		command     *CopyCommand
	}
	testCases := []testCase{
		func() testCase {
			sd := make([]string, 0)
			c := &CopyCommand{
				cmd: &instructions.CopyCommand{
					SourcesAndDest: sd,
				},
				BaseCommand: BaseCommand{
					img: fakeImage{
						ImageLayers: []v1.Layer{
							fakeLayer{TarContent: tarContent},
						},
					},
					cached: true,
				},
			}
			count := 0
			tc := testCase{
				desctiption: "cached with valid image and valid layer",
				count:       &count,
				expectLayer: true,
			}
			c.extractFn = func(_ string, _ *tar.Header, _ io.Reader) error {
				*tc.count++
				return nil
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CopyCommand{
				cmd: &instructions.CopyCommand{},
				BaseCommand: BaseCommand{
					cached: true,
				},
			}
			tc := testCase{
				desctiption: "cached with no image",
				expectErr:   true,
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CopyCommand{
				cmd: &instructions.CopyCommand{},
				BaseCommand: BaseCommand{
					img:    fakeImage{},
					cached: true,
				},
			}
			tc := testCase{
				desctiption: "cached with image containing no layers",
				expectErr:   true,
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CopyCommand{
				cmd: &instructions.CopyCommand{},
				BaseCommand: BaseCommand{
					img: fakeImage{
						ImageLayers: []v1.Layer{
							fakeLayer{},
						},
					},
					cached: true,
				},
			}
			tc := testCase{
				desctiption: "cached with image one layer which has no tar content",
				expectErr:   false, // this one probably should fail but doesn't because of how ExecuteCommand and util.GetFSFromLayers are implemented - cvgw- 2019-11-25
				expectLayer: true,
			}
			tc.command = c
			return tc
		}(),
	}

	for _, tc := range testCases {
		t.Run(tc.desctiption, func(t *testing.T) {
			c := tc.command
			err := c.ExecuteCommand(config, buildArgs)
			if !tc.expectErr && err != nil {
				t.Errorf("Expected err to be nil but was %v", err)
			} else if tc.expectErr && err == nil {
				t.Error("Expected err but was nil")
			}

			if tc.count != nil && *tc.count != 1 {
				t.Errorf("Expected extractFn to be called %v times but was called %v times", 1, *tc.count)
			}

			if c.layer == nil && tc.expectLayer {
				t.Error("expected the command to have a layer set but instead was nil")
			} else if c.layer != nil && !tc.expectLayer {
				t.Error("expected the command to have no layer set but instead found a layer")
			}

			if c.readSuccess != tc.expectLayer {
				t.Errorf("expected read success to be %v but was %v", tc.expectLayer, c.readSuccess)
			}
		})
	}
}
