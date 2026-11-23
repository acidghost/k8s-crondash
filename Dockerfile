FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder
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
