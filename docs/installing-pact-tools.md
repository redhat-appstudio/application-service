# Installing Pact Tools

The Pact tests in the controller package require pact tooling to be installed and on your path. Follow these instructions to do so:

1. Change directory to an appropriate folder (e.g. `/usr/local`)
2. Run `curl -fsSL https://raw.githubusercontent.com/pact-foundation/pact-ruby-standalone/master/install.sh | bash`
3. Add the pact tools' bin folder (e.g. `/usr/local/pact/bin`) to your path to your shell PATH. Ensure all binary files within the `bin/` folder has executable permissions
4. Run `go install github.com/pact-foundation/pact-go@v1` to install the `pact-go` tool
5. Run `pact-go install` to validate that all of the necessary Pact tools are installed
