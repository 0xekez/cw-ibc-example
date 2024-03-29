optimize:
    if [[ $(uname -m) =~ "arm64" ]]; then \
    docker run --rm -v "$(pwd)":/code \
        --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
        --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
        --platform linux/arm64 \
        cosmwasm/rust-optimizer-arm64:0.12.12; else \
    docker run --rm -v "$(pwd)":/code \
        --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
        --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
        --platform linux/amd64 \
        cosmwasm/rust-optimizer:0.12.12; fi

simtest: optimize
    mkdir -p tests/wasms
    if [[ $(uname -m) =~ "arm64" ]]; then cp artifacts/cw_ibc_example-aarch64.wasm tests/wasms/cw_ibc_example.wasm ; else cp artifacts/cw_ibc_example.wasm tests/wasms; fi
    cd tests/simtests && go test ./...

interchaintest: optimize
    mkdir -p tests/wasms
    if [[ $(uname -m) =~ "arm64" ]]; then cp artifacts/cw_ibc_example-aarch64.wasm tests/wasms/cw_ibc_example.wasm ; else cp artifacts/cw_ibc_example.wasm tests/wasms; fi
    cd tests/strangelove && go test ./...
