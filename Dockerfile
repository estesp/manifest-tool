FROM golang:1.24

RUN go install golang.org/x/tools/cmd/cover@latest \
    && go install golang.org/x/lint/golint@latest

ENV REGISTRY_COMMIT=a4d9db5a884b70be0c96dd6a7a9dbef4f2798c51
RUN set -x \
	&& mkdir -p /go/src/github.com/distribution && cd /go/src/github.com/distribution \
	&& git clone https://github.com/distribution/distribution.git && cd distribution \
	&& git checkout "$REGISTRY_COMMIT" \
	&& make binaries && cp bin/registry /usr/local/bin

# The source is bind-mounted into this folder
WORKDIR /go/src/github.com/estesp/manifest-tool
