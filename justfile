program := 'k8s-crondash'

container_engine := 'docker'
container_registry := 'ghcr.io'
container_image := container_registry + '/acidghost/' + program

version := 'SNAPSHOT-'+`git describe --tags --always --dirty 2>/dev/null || printf 'unknown'`
commit_sha := `(git rev-parse HEAD 2>/dev/null || printf 'unknown') | tr -d '\n'`
build_time := `date -u '+%Y-%m-%d_%H:%M:%S'`

ldflags := '-s -w -X main.buildVersion='+version \
        +' -X main.buildCommit='+commit_sha \
        +' -X main.buildDate='+build_time

goos := if os() == 'macos' { 'darwin' } else { os() }
goarch := if arch() == 'aarch64' { 'arm64' } else if arch() == 'x86_64' { 'amd64' } else { arch() }

helm_flags := '--set auth.username=user,auth.password=changeme'

alias b := build
alias r := run

help:
    @just --list

generate:
    go tool templ generate

build-all: (build 'darwin' 'arm64') (build 'linux' 'arm64') (build 'linux' 'amd64')

build os=goos arch=goarch: generate build-dir
    CGO_ENABLED=0 GOOS={{os}} GOARCH={{arch}} \
        go build \
            -ldflags '{{ldflags}}' \
            -o build/{{program}}-{{os}}-{{arch}}

build-dir:
    mkdir -p build

run *args: build
    ./build/{{program}}-{{goos}}-{{goarch}} {{args}}

build-image:
    {{container_engine}} build \
        --build-arg "BUILD_VERSION={{version}}" \
        --build-arg "BUILD_COMMIT={{commit_sha}}" \
        -t {{container_image}} .

run-image *args: build-image
    {{container_engine}} run --rm {{container_image}} {{args}}

helm-lint:
    helm lint {{helm_flags}} deploy/charts/{{program}}

helm-conform:
    helm template {{helm_flags}} {{program}} deploy/charts/{{program}} \
        | kubeconform -summary

vendor:
    go mod tidy
    go mod vendor

fmt:
    go tool templ fmt .
    go fmt ./...

lint:
    golangci-lint run

test:
    go test ./...

install: build
    cp -v './build/{{program}}-{{goos}}-{{goarch}}' "$(go env GOBIN)/{{program}}"

clean:
    rm -rf build
