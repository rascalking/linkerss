FROM golang
ADD . /go/src/github.com/rascalking/linkerss
RUN go get \
	github.com/coreos/pkg/flagutil \
	github.com/dghubble/go-twitter/twitter \
	github.com/gorilla/feeds \
	golang.org/x/oauth2 \
	golang.org/x/net/html
RUN go install github.com/rascalking/linkerss
CMD /go/bin/linkerss
EXPOSE 9999
