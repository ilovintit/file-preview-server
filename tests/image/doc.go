// Package image holds the镜像级端到端验证：它只在 `image` 构建标签下编译，
// 由 tests/image/run.sh 在实际构建出的应用镜像上执行，不参与生产构建。
package image
