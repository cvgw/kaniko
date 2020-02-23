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

package integration

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/kaniko/pkg/timing"
)

func CreateContextTarball(contextFile, dir string) (string, error) {
	log.Println("Creating tarball of integration test files to use as build context")

	cmd := exec.Command("tar", "-C", dir, "-zcvf", contextFile, ".")
	_, err := RunCommandWithoutTest(cmd)
	if err != nil {
		return "", fmt.Errorf("Failed to create build context tarball from integration dir: %s", err)
	}
	return contextFile, err
}

func buildImageWithLocalContext(config *integrationTestConfig, contextDir, testDesc string) error {
	serviceAccount, imageRepo := config.serviceAccount, config.imageRepo

	fmt.Printf("Building images for test %s\n", testDesc)

	var buildArgs []string
	buildArgFlag := "--build-arg"
	for _, arg := range argsMap[testDesc] {
		buildArgs = append(buildArgs, buildArgFlag, arg)
	}
	// build docker image
	additionalFlags := append(buildArgs, additionalDockerFlagsMap[testDesc]...)
	dockerImage := strings.ToLower(imageRepo + dockerPrefix + testDesc)
	dockerCmd := exec.Command("docker",
		append([]string{"build",
			"-t", dockerImage,
			contextDir},
			additionalFlags...)...,
	)
	if env, ok := envsMap[testDesc]; ok {
		dockerCmd.Env = append(dockerCmd.Env, env...)
	}

	timer := timing.Start(testDesc + "_docker")
	out, err := RunCommandWithoutTest(dockerCmd)
	timing.DefaultRun.Stop(timer)
	if err != nil {
		return fmt.Errorf("Failed to build image %s with docker command \"%s\": %s %s", dockerImage, dockerCmd.Args, err, string(out))
	}
	fmt.Printf("Build image for test %s as %s. docker build output: %s \n", testDesc, dockerImage, out)

	// build kaniko image
	additionalFlags = append(buildArgs, additionalKanikoFlagsMap[testDesc]...)
	kanikoImage := GetKanikoImage(imageRepo, testDesc)
	fmt.Printf("Going to build image with kaniko: %s, flags: %s \n", kanikoImage, additionalFlags)
	dockerRunFlags := []string{"run", "--net=host",
		"-v", contextDir + ":/workspace",
	}
	if env, ok := envsMap[testDesc]; ok {
		for _, envVariable := range env {
			dockerRunFlags = append(dockerRunFlags, "-e", envVariable)
		}
	}
	dockerRunFlags = addServiceAccountFlags(dockerRunFlags, serviceAccount)

	dockerRunFlags = append(dockerRunFlags, ExecutorImage,
		"-d", kanikoImage,
		fmt.Sprintf("--context=dir://%s", "/workspace"),
	)
	dockerRunFlags = append(dockerRunFlags, additionalFlags...)

	kanikoCmd := exec.Command("docker", dockerRunFlags...)

	timer = timing.Start(testDesc + "_kaniko")
	out, err = RunCommandWithoutTest(kanikoCmd)
	timing.DefaultRun.Stop(timer)
	if err != nil {
		return fmt.Errorf("Failed to build image %s with kaniko command \"%s\": %s %s", dockerImage, kanikoCmd.Args, err, string(out))
	}
	if outputCheck := outputChecks[testDesc]; outputCheck != nil {
		if err := outputCheck(testDesc, out); err != nil {
			return fmt.Errorf("Output check failed for image %s with kaniko command \"%s\": %s %s", dockerImage, kanikoCmd.Args, err, string(out))
		}
	}

	return nil
}

func TestWithContext(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(cwd, "dockerfiles-with-context")

	testDirs, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, tdInfo := range testDirs {
		name := tdInfo.Name()
		testDir := filepath.Join(dir, name)

		t.Run("test_"+name, func(t *testing.T) {
			t.Parallel()

			dockerfile := name

			if err := buildImageWithLocalContext(config, testDir, dockerfile); err != nil {
				t.Errorf("Error building image: %s", err)
				t.FailNow()
			}

			dockerImage := GetDockerImage(config.imageRepo, dockerfile)
			kanikoImage := GetKanikoImage(config.imageRepo, dockerfile)

			// container-diff
			daemonDockerImage := daemonPrefix + dockerImage
			containerdiffCmd := exec.Command("container-diff", "diff", "--no-cache",
				daemonDockerImage, kanikoImage,
				"-q", "--type=file", "--type=metadata", "--json")
			diff := RunCommand(containerdiffCmd, t)
			t.Logf("diff = %s", string(diff))

			expected := fmt.Sprintf(emptyContainerDiff, dockerImage, kanikoImage, dockerImage, kanikoImage)
			checkContainerDiffOutput(t, diff, expected)

		})
	}

	if err := logBenchmarks("benchmark"); err != nil {
		t.Logf("Failed to create benchmark file: %v", err)
	}
}
