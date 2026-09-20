// Command probe 打印一次 HTTPS 探针请求的真实状态码，供镜像级编排脚本判断
// 就绪与依赖故障行为。连接失败打印 000，不掩盖失败。
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	target := flag.String("url", "", "探针 URL")
	ca := flag.String("ca", "", "一次性 CI 根证书")
	flag.Parse()
	if *target == "" || *ca == "" {
		fmt.Fprintln(os.Stderr, "缺少 -url 或 -ca")
		os.Exit(2)
	}
	pem, err := os.ReadFile(*ca)
	pool := x509.NewCertPool()
	if err != nil || !pool.AppendCertsFromPEM(pem) {
		fmt.Fprintln(os.Stderr, "无法读取一次性 CI 根证书")
		os.Exit(2)
	}
	client := &http.Client{
		Timeout:       10 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Get(*target)
	if err != nil {
		fmt.Println("000")
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	fmt.Println(response.StatusCode)
}
