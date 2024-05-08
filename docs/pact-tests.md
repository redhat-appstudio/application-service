# HAS contract testing using Pact 

HAS is a participant in contract testing within RHTAP using [Pact](https://pact.io/) framework. It is a provider of an API that is used by HAC-dev. This documentation is about the specifics of provider testing on HAS side. If you want to know more about contract testing using Pact, follow the [official documentation](https://docs.pact.io/). For more information about the RHTAP contract testing, follow the documentation in the [HAC-dev repo](https://github.com/openshift/hac-dev/blob/main/pactTests.md).

## Table of content
  - [When does test run](#when-does-test-run)
  - [Implementation details](#implementation-details)
  - [Adding a new verification](#adding-a-new-verification)
  - [Failing test](#failing-test)

## When does test run
Pact tests are triggered during different life phases of a product. See the table below for the detail.

| Event       | What is checked | Pushed to Pact broker | Implemented |
|-------------|-----------------|-----------------------|-------------|
| Locally as part of unit tests<br />`make test` | Runs verification against <br />consumer "main" branch | No | Yes |
| PR update   | Runs verification against consumer  <br />"main" branch and all environments | No* | Yes** [link](https://github.com/konflux-ci/application-service/blob/main/.github/workflows/pr.yml#L124) |
| PR merge | ? | Yes<br />commit SHA is a version <br />tagged by branch "main" | No |

\* The idea was to push also those tags, but for now, nothing is pushed as we don't have access to the secrets from this GH action.

\*\* Currently, we don't have any environment specified. There should be a "staging" and "production" environment in the future. 

For more information, follow the [HAC-dev documentation](https://github.com/openshift/hac-dev/blob/main/pactTests.md) also with the [Gating](https://github.com/openshift/hac-dev/blob/main/pactTests.md#gating) chapter.


## Implementation details
Pact tests live in a `controllers` folder. The main test file is an `application_pact_tests.go`. The Pact setup is done there, including the way to obtain the contracts. By default, contracts are downloaded from the Pact broker. If you are developing the new tests and want to specify the contract file locally, there is a section commented out that helps you with the setup. For more information, follow the [officital documentation](https://docs.pact.io/implementation_guides/go/readme#provider-verification).

The definition of StateHandlers is one of the most important parts of the test setup. They define the state of the provider before the request from the contract is executed. The string defining the state have to be the same as the `providerState` field of the contract. Methods implementing a state are extracted to the `application_pact_test_state_handlers.go` file. 

The place where the tests are executed is the line 
```
// Run pact tests
_, err = pact.VerifyProvider(t, verifyRequest)
```
Pact itself is taking care of downloading the contracts, executing the tests, generating results, and (if configured) pushing them to the Pact broker.

## Adding a new verification
When a new contract is created, it would probably include a new state that is not implemented yet on the provider side. Although the Pact tests would probably fail because of the undefined state, an error message may be misleading. Instead of telling you that the state is not defined, the pact just skips the state and executes the request which is probably going to fail. You can see this message in the log:
```
[WARN] state handler not found for state: <new state>
[DEBUG] skipping state handler for request <request>

```

To implement the state, add the string from the contract to the `setup state handlers` block.
```
verifyRequest.StateHandlers = pactTypes.StateHandlers{
        "No app with the name myapp in the default namespace exists.":        func() error { return nil },
        "App myapp exists and has component gh-component and quay-component": <-createAppAndComponents(HASAppNamespace),
    }
```
Add implementation of this state to the `application_pact_test_state_handlers.go`. Consider adding some logic to the `AfterEach` block if needed. That's it!

## Failing test
Pact tests are running locally as part of the unit tests, so you may find them failing once the breaking change is made. If you're not sure why the test is failing, feel free to ping kfoniok.
The best way to deal with the failing contract test is to fix the code change to make it compatible with the contract again. If it is not possible, then you should ping someone from the HAC-dev team to make them aware that the breaking change is coming. If you decide to push the code to the PR with the failing contract tests, it should fail the Pact PR check job there too.
Verification results are not pushed to the Pact broker until the PR is merged. (TBD) PR should be merged only when Pact tests are passing.