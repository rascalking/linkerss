FROM golang
ADD . /go/src/github.com/rascalking/linkerss
RUN go get \
	    github.com/coreos/pkg/flagutil \
	    github.com/dghubble/go-twitter/twitter \
	    github.com/gorilla/feeds \
	    golang.org/x/net/html \
	    golang.org/x/oauth2
RUN go install github.com/rascalking/linkerss
ENTRYPOINT /go/bin/linkerss
EXPOSE 9999
