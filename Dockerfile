FROM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder
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
