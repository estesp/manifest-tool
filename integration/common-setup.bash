#!/usr/bin/env bash

function _common_setup() {
    # get the containing directory of this file
    # use $BATS_TEST_FILENAME instead of ${BASH_SOURCE[0]} or $0,
    # as those will point to the bats executable's location or the preprocessed file respectively
    PROJECT_ROOT="$( cd "$( dirname "$BATS_TEST_FILENAME" )/.." >/dev/null 2>&1 && pwd )"

    RUNTIME_TOOL="${RUNTIME_TOOL:-finch}"
}

function _load_test_images() {

    "${RUNTIME_TOOL}" pull --platform linux/amd64 amd64/alpine:latest
    "${RUNTIME_TOOL}" pull --platform linux/arm64/v8 arm64v8/alpine:latest
    "${RUNTIME_TOOL}" pull --platform linux/ppc64le ppc64le/alpine:latest
    "${RUNTIME_TOOL}" pull --platform linux/s390x s390x/alpine:latest
    "${RUNTIME_TOOL}" tag amd64/alpine:latest ${HOSTNM}/alpine:amd64
    "${RUNTIME_TOOL}" tag ppc64le/alpine:latest ${HOSTNM}/alpine:ppc64le
    "${RUNTIME_TOOL}" tag arm64v8/alpine:latest ${HOSTNM}/alpine:arm64
    "${RUNTIME_TOOL}" tag s390x/alpine:latest ${HOSTNM}/alpine:s390x
    "${RUNTIME_TOOL}" push ${HOSTNM}/alpine:amd64
    "${RUNTIME_TOOL}" push ${HOSTNM}/alpine:arm64
    "${RUNTIME_TOOL}" push ${HOSTNM}/alpine:ppc64le
    "${RUNTIME_TOOL}" push ${HOSTNM}/alpine:s390x
}
