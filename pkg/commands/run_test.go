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
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	"github.com/GoogleContainerTools/kaniko/testutil"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func Test_addDefaultHOME(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		mockUser *user.User
		initial  []string
		expected []string
	}{
		{
			name: "HOME already set",
			user: "",
			initial: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
			expected: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
		},
		{
			name: "HOME not set and user not set",
			user: "",
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
		{
			name: "HOME not set and user and homedir for the user set",
			user: "www-add",
			mockUser: &user.User{
				Username: "www-add",
				HomeDir:  "/home/some-other",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/home/some-other",
			},
		},
		{
			name: "HOME not set and user set",
			user: "www-add",
			mockUser: &user.User{
				Username: "www-add",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/home/www-add",
			},
		},
		{
			name: "HOME not set and user is set",
			user: "newuser",
			mockUser: &user.User{
				Username: "newuser",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/home/newuser",
			},
		},
		{
			name: "HOME not set and user is set to root",
			user: "root",
			mockUser: &user.User{
				Username: "root",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userLookup = func(username string) (*user.User, error) { return test.mockUser, nil }
			defer func() { userLookup = user.Lookup }()
			actual := addDefaultHOME(test.user, test.initial)
			testutil.CheckErrorAndDeepEqual(t, false, nil, test.expected, actual)
		})
	}
}

func prepareTarFixture() ([]byte, error) {
	dir, err := ioutil.TempDir("/tmp", "tar-fixture")
	if err != nil {
		return nil, err
	}

	content := `
Meow meow meow meow
meow meow meow meow
`
	if err := ioutil.WriteFile(filepath.Join(dir, "foo.txt"), []byte(content), 0777); err != nil {
		return nil, err
	}

	writer := bytes.NewBuffer([]byte{})
	tw := tar.NewWriter(writer)
	defer tw.Close()
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := tw.Write(body); err != nil {
			log.Fatal(err)
		}
		if info.IsDir() {
			return nil
		}

		return nil
	})

	return writer.Bytes(), nil
}

func Test_CachingRunCommand_ExecuteCommand(t *testing.T) {
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
		command     *CachingRunCommand
	}
	testCases := []testCase{
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{
					ImageLayers: []v1.Layer{
						fakeLayer{TarContent: tarContent},
					},
				},
			}
			count := 0
			tc := testCase{
				desctiption: "with valid image and valid layer",
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
			c := &CachingRunCommand{}
			tc := testCase{
				desctiption: "with no image",
				expectErr:   true,
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{},
			}
			tc := testCase{
				desctiption: "with image containing no layers",
				expectErr:   true,
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{
					ImageLayers: []v1.Layer{
						fakeLayer{},
					},
				},
			}
			tc := testCase{
				desctiption: "with image one layer which has no tar content",
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
