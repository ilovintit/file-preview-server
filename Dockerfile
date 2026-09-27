FROM reg.shw.top/ci-cache/shw-plugin-toolchain@sha256:4b71bd74caa70a65cde2120fa8438dd141a1afe7c8268367cbc76ab202d80b9e AS build
WORKDIR /src
COPY go.mod go.sum ./
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local
RUN --mount=type=secret,id=go_proxy test -s /run/secrets/go_proxy && GOPROXY="$(cat /run/secrets/go_proxy)" go mod download
COPY . .
RUN --mount=type=secret,id=go_proxy GOPROXY="$(cat /run/secrets/go_proxy)" go build -trimpath -ldflags="-s -w" -o /out/file-preview-server ./
# playground 只在开发环境部署，单独成像；生产镜像不包含它，也没有 /demo/ 路由。
RUN --mount=type=secret,id=go_proxy GOPROXY="$(cat /run/secrets/go_proxy)" go build -trimpath -ldflags="-s -w" -o /out/file-preview-playground ./demo/playground

FROM reg.shw.top/library/alpine@sha256:dabf91b69c191a1a0a1628fd6bdd029c0c4018041c7f052870bb13c5a222ae76 AS runtime
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
