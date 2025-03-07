#!/usr/bin/env bats


setup_file() {
    load 'common-setup'
    _common_setup

    # start a container registry
    "${RUNTIME_TOOL}" run -d -p 5000:5000 --name registry registry:2
    export HOSTNM="localhost:5000"
    _load_test_images
}

teardown_file() {
    # stop our registry
    "${RUNTIME_TOOL}" stop registry
    "${RUNTIME_TOOL}" rm registry
}

@test "can run manifest-tool version" {
    ./manifest-tool --version
}

@test "can inspect a basic image" {
    ./manifest-tool --plain-http inspect ${HOSTNM}/alpine:amd64
}
