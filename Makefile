.PHONY: all binary build static clean install install shell test-integration

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")
DOCKER_IMAGE := manifest-tool-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
# set env like gobuildtag?
DOCKER_RUN := docker run --rm -i #$(DOCKER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_RUN += -t
endif
DOCKER_RUN_DOCKER := $(DOCKER_RUN) -v $(shell pwd):/go/src/github.com/estesp/manifest-tool -w /go/src/github.com/estesp/manifest-tool "$(DOCKER_IMAGE)"

all: binary

build:
	$(DOCKER_RUN) -v $(shell pwd):/go/src/github.com/estesp/manifest-tool -w /go/src/github.com/estesp/manifest-tool golang:1.13 /bin/bash -c "\
		go build -ldflags \"-X main.gitCommit=${COMMIT}\" -o manifest-tool github.com/estesp/manifest-tool"

# Target to build a dynamically linked binary
binary:
	go build -ldflags "-X main.gitCommit=${COMMIT}"

# Target to build a statically linked binary
static:
	GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 go build \
	   -ldflags "-w -extldflags -static -X main.gitCommit=${COMMIT}" \
	   -tags netgo -installsuffix netgo

build-container:
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)" .

clean:
	rm -f manifest-tool

cross:
	hack/cross.sh

cross-clean:
	rm -f manifest-tool-*

signrelease:
	hack/sign-release.sh

install:
	install -d -m 0755 ${INSTALLDIR}
	install -m 755 manifest-tool ${INSTALLDIR}

shell: build-container
	$(DOCKER_RUN_DOCKER) bash

validate: build-container
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet
