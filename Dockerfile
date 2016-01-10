# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/rascalking/linkerss

# Build the outyet command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
RUN go get \
    github.com/coreos/pkg/flagutil \
    github.com/dghubble/go-twitter/twitter \
    golang.org/x/oauth2
RUN go install github.com/rascalking/linkerss

# Run the outyet command by default when the container starts.
ENTRYPOINT /go/bin/linkerss
