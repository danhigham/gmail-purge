FROM golang:1.11-alpine

WORKDIR /go/src/app
COPY purge.go .

RUN apk add git
RUN go get -d -v ./...
RUN go install -v ./...

CMD ["app"]
