FROM golang:latest
RUN go get -v github.com/jessevdk/go-assets-builder
CMD make
