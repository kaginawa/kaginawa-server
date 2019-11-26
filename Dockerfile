FROM golang:1-alpine as build_base
RUN apk add git
WORKDIR /go/src/github.com/kaginawa/kaginawa-server
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on go mod download

FROM build_base AS builder
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go install -a -v ./...

FROM alpine AS server
RUN apk add ca-certificates
COPY --from=builder /go/bin/server /bin/server
COPY --from=builder /go/src/github.com/kaginawa/kaginawa-server/assets assets
COPY --from=builder /go/src/github.com/kaginawa/kaginawa-server/template template
ENTRYPOINT ["/bin/server"]
