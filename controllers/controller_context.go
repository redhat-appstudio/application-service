package controllers

import gh "github.com/google/go-github/v41/github"

// Common context for application service reconsilers.
type ControllerContext struct {
	GithubConf *GithubConfiguration
}

// Github properties required to execute application pipeline
type GithubConfiguration struct {
	GithubClient *gh.Client
	GithubOrg    string
	GithubToken  string
}
