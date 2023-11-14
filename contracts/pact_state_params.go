//
// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package contracts

// !!!
//
// Parameters have to correspond with the https://github.com/openshift/hac-dev/blob/main/pact-tests/states/state-params.ts
//
// !!!

// ComponentParams are the Pact parameters used for component manipulation.
// app - describes the Application tnat component belongs to
// repo - Git repository for the component
// name - name of the component
type ComponentParams struct {
	app  ApplicationParams
	repo string
	name string
}

// ApplicationParams are the Pact parameters used for application manipulation.
// appName - name of the application
// namespace - namespace where the application should live, currently only "default" is supported
type ApplicationParams struct {
	appName   string
	namespace string
}

func parseApplication(params map[string]interface{}) ApplicationParams {
	return ApplicationParams{
		params["params"].(map[string]interface{})["appName"].(string),
		params["params"].(map[string]interface{})["namespace"].(string),
	}
}

func parseComponents(params map[string]interface{}) []ComponentParams {
	tmp := params["params"].(map[string]interface{})["components"].([]interface{})
	var components []ComponentParams
	for _, compToParse := range tmp {
		component := compToParse.(map[string]interface{})
		appParsed := ApplicationParams{component["app"].(map[string]interface{})["appName"].(string),
			component["app"].(map[string]interface{})["namespace"].(string)}
		compParsed := ComponentParams{appParsed, component["repo"].(string), component["compName"].(string)}
		components = append(components, compParsed)
	}
	return components
}
