FROM golang:1.26.4-alpine@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS builder
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
