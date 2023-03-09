# Tests

Broadly speaking, there are two approaches to testing IBC smart contracts.

1. [`ts-relayer`](https://github.com/confio/cw-ibc-demo/tree/main/tests),
   which uses Confio's IBC relayer and cosmjs package to write
   Javascript tests for IBC contracts deployed on two docker chains
   running locally.
2. Simulation testing, which uses the SDK's [simulation
   framework](https://docs.cosmos.network/main/core/simulation) to
   spin up multiple simulated chains and relay IBC packets between
   them.

You might prefer `ts-relayer` if you want a close-to-exact,
dockerized, chain setup. You might prefer simulation testing if you
want tests that run quickly, against real SDK go code.

I think that any real CosmWasm IBC code should use both testing methods
depending on the type of test being written.

## Simulation testing

The `simtests` subdirectory contains an example setup for simulated
IBC testing. `simtests/can_count_test.go` is well commented and
intended to be read as a "tutorial".

### Setting up simulation tests

The wasmd repository has [e2e
tests](https://github.com/CosmWasm/wasmd/tree/main/tests/e2e) which
test IBC connections using the simulation testing framework. To add
simulation tests to your own contract, I recomend:

1. Copy the
   [`go.mod`](https://github.com/CosmWasm/wasmd/blob/main/go.mod) file
   from the e2e tests to your testing directory.
2. Copy the
   [`ibc_fees_test.go`](https://github.com/CosmWasm/wasmd/blob/main/tests/e2e/ibc_fees_test.go)
   file into a new "simtests" folder in your project.
3. Change `package e2e` to `package simtests` in the copied file.
4. Run `go get -t ./...` to download your dependencies.
5. Run `go mod tidy` to remove all the dependencies you don't need from `wasmd`.
6. Delete the `ibc_fees_test.go` file and start writing your own simtests!

The [justfile](../justfile) in this repository contains an example of
how you can wire up a system for automatically compiling your
contracts, and running simulation tests against them.

[`simulation.yml`](../.github/workflows/simulation.yml) contains an
example GitHub Actions workflow to run these simulation tests in CI.
