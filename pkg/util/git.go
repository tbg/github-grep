// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package util

import (
	"os/exec"

	"regexp"

	"bytes"

	"github.com/pkg/errors"
)

var repoRE = regexp.MustCompile(`github\.com(?:\/|:)([^\/]+)\/(.*).git$`)

// Repo loads the Github organization and repository name from the specified
// directory and upstream.
func Repo(dir string, remote string) (org string, repo string, _ error) {
	if remote == "" {
		remote = "origin"
	}
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	b = bytes.TrimSpace(b)
	if err != nil {
		return "", "", errors.Wrapf(
			err,
			"unable to load repository information for directory '%s': %s", dir, string(b),
		)
	}
	matches := repoRE.FindStringSubmatch((string(b)))
	if len(matches) != 3 {
		return "", "", errors.Errorf(
			"unable to detect GitHub organization and repo from '%s'", string(b),
		)
	}
	return matches[1], matches[2], nil
}
