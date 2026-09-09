FROM reg.shw.top/ci-cache/ci-golang@sha256:8e736350aeea4d3fcc8fcfab715c34a7acca72d63e79c87927d59660a3d77c37 AS go
FROM reg.shw.top/ci-cache/ci-playwright-toolchain@sha256:e7732b466aa4abbc88c03c5779e7fa3305fce697c63e803c08c7462a74774f32
COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}" GOTOOLCHAIN=local CGO_ENABLED=0
RUN test "$(go env GOVERSION)" = go1.25.14 && node --version && python3 --version
