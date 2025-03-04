FROM golang:1-alpine AS builder
RUN apk add git
WORKDIR /go/src/github.com/kaginawa/kaginawa-server
COPY . .
RUN CGO_ENABLED=0 go install -a -v ./...

FROM alpine AS server
RUN apk add ca-certificates
COPY --from=builder /go/bin/server /bin/server
COPY --from=builder /go/src/github.com/kaginawa/kaginawa-server/assets assets
COPY --from=builder /go/src/github.com/kaginawa/kaginawa-server/template template
ENTRYPOINT ["/bin/server"]
