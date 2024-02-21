FROM golang:1.19-bullseye AS build

RUN apt update && update-ca-certificates --fresh && apt -y install build-essential

#build auditor
WORKDIR /go/src/build
COPY ./ /go/src/build
RUN CGO_ENABLED=0 go build -ldflags "-s -w"

FROM debian:bullseye-slim

COPY --from=build /go/src/build/argo-sidecar-helm-injector /usr/local/bin

ENTRYPOINT ["/usr/local/bin/argo-sidecar-helm-injector"]
