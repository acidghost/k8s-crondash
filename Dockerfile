FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder
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
