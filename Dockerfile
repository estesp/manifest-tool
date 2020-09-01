FROM golang:1.14-alpine3.12 as build-env

#ENV GO_TOOLS_COMMIT 823804e1ae08dbb14eb807afc7db9993bc9e3cc3
# Grab Go's cover tool for dead-simple code coverage testing
# Grab Go's vet tool for examining go code to find suspicious constructs
# and help prevent errors that the compiler might not catch
#RUN git clone https://github.com/golang/tools.git /go/src/golang.org/x/tools \
#	&& (cd /go/src/golang.org/x/tools && git checkout -q $GO_TOOLS_COMMIT) \
#	&& go install -v golang.org/x/tools/cmd/cover \
#	&& go install -v golang.org/x/tools/cmd/vet
# Grab Go's lint tool
#ENV GO_LINT_COMMIT 32a87160691b3c96046c0c678fe57c5bef761456
#RUN git clone https://github.com/golang/lint.git /go/src/github.com/golang/lint \
#	&& (cd /go/src/github.com/golang/lint && git checkout -q $GO_LINT_COMMIT) \
#	&& go install -v github.com/golang/lint/golint
#	&& git clone https://github.com/docker/distribution.git "$GOPATH/src/github.com/docker/distribution" \
#	&& (cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT") \
	#&& GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
	#go build -o /usr/local/bin/registry github.com/docker/distribution/cmd/registry \

COPY . /go

RUN apk update && apk add make git gcc musl-dev

#ENV REGISTRY_COMMIT 47a064d4195a9b56133891bbb13620c3ac83a827
RUN set -x && export GOPATH=$HOME/go && go mod download && make binary
# && make static

# The source is bind-mounted into this folder

FROM alpine:3.12

COPY --from=build-env /go/manifest-tool /usr/local/bin/

CMD /usr/local/bin/manifest-tool

