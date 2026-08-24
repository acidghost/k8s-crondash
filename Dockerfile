FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder
RUN apk add --no-cache git just
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
ARG BUILD_VERSION=0.0.0
ARG BUILD_COMMIT=unknown
RUN just version="${BUILD_VERSION}" commit_sha="${BUILD_COMMIT}" build \
    && mv build/k8s-crondash-linux-* /bin/k8s-crondash

FROM scratch
USER 65534:65534
COPY --from=builder /bin/k8s-crondash /bin/k8s-crondash
EXPOSE 3000
ENTRYPOINT ["k8s-crondash"]
