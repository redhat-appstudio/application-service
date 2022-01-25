/*
Copyright 2022 Red Hat, Inc.

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

package controllers

import gh "github.com/google/go-github/v41/github"

// Common context for application service reconsilers.
type ControllerContext struct {
	// Github specific configuration
	GithubConf *GithubConfiguration
	// Namespace where is deployed operator. Notice: That's not the same with "WatchNamespace".
	Namespace string
}

// Github properties required to execute application pipeline
type GithubConfiguration struct {
	GithubClient *gh.Client
	GithubOrg    string
	GithubToken  string
}
