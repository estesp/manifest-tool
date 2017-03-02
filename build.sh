#!/usr/bin/env bash

GOPATH="${HOME}/go"
GIT_UPSTREAM_REPOSITORY="https://github.com/estesp/manifest-tool"

function makeBuild {
    cd ${GOPATH}/src

    if [ ! -d "github.com/estesp" ]; then
        mkdir -p "github.com/estesp"
    fi

    cd "github.com/estesp"

    if [ ! -d "manifest-tool" ]; then
        git clone "${GIT_UPSTREAM_REPOSITORY}"
    else
        git pull
    fi

    cd "manifest-tool" && make binary && sudo make install
}

if [ -d "${GOPATH}/src" ]; then
    echo "manifest_install.sh: src found, builing..."
    makeBuild
else
    echo "manifest_install.sh: src not found, creating..."
    mkdir -p "${GOPATH}/src"
    makeBuild
fi


