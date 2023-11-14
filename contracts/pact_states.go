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

// The list of used states and their params can be found there: https://github.com/openshift/hac-dev/blob/main/pact-tests/states/states.ts#L18

import (
	models "github.com/pact-foundation/pact-go/v2/models"
)

func setupStateHandler() models.StateHandlers {
	return models.StateHandlers{
		"Application exists":         createApp,
		"Application has components": createComponents,
	}
}
