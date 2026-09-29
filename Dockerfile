FROM golang:1.25.14-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437 AS build
WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/file-preview-server ./
# playground 只用于本地演示，单独成像；发布镜像不包含它，也没有 /demo/ 路由。
RUN go build -trimpath -ldflags="-s -w" -o /out/file-preview-playground ./demo/playground

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN addgroup -S preview && adduser -S -G preview -H -s /sbin/nologin preview
USER preview

FROM runtime AS playground
COPY --from=build /out/file-preview-playground /usr/local/bin/file-preview-playground
EXPOSE 8443
ENTRYPOINT ["/usr/local/bin/file-preview-playground"]

FROM runtime AS server
COPY --from=build /out/file-preview-server /usr/local/bin/file-preview-server
EXPOSE 9501
ENTRYPOINT ["/usr/local/bin/file-preview-server"]
CMD ["server"]
