// Command prepare 为镜像级端到端验证准备一次性材料：隔离的 CI 证书、
// 隔离的 silo bucket 和随机 fixture 凭据。它只在 CI 的一次性网络里运行，
// 不接触任何业务环境，也不写入仓库跟踪文件。
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"time"

	s3 "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	out := flag.String("out", ".cache/image-e2e", "输出目录")
	endpoint := flag.String("silo-endpoint", "", "silo S3 endpoint")
	access := flag.String("silo-access-key", "", "silo access key")
	secret := flag.String("silo-secret-key", "", "silo secret key")
	bucket := flag.String("bucket", "", "一次性 bucket 名称")
	flag.Parse()
	if *endpoint == "" || *access == "" || *secret == "" || *bucket == "" {
		fail(fmt.Errorf("缺少 silo fixture 配置"))
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	caCert, caKey, err := authority()
	if err != nil {
		fail(err)
	}
	if err := writePEM(filepath.Join(*out, "ca.pem"), "CERTIFICATE", caCert.Raw); err != nil {
		fail(err)
	}
	for name, host := range map[string]string{"app": "app", "harness": "harness"} {
		cert, key, err := leaf(caCert, caKey, host)
		if err != nil {
			fail(err)
		}
		if err := writePEM(filepath.Join(*out, name+"-cert.pem"), "CERTIFICATE", cert); err != nil {
			fail(err)
		}
		encoded, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			fail(err)
		}
		if err := writePEM(filepath.Join(*out, name+"-key.pem"), "EC PRIVATE KEY", encoded); err != nil {
			fail(err)
		}
	}
	u, err := url.Parse(*endpoint)
	if err != nil {
		fail(err)
	}
	api, err := s3.New(u.Host, &s3.Options{Creds: credentials.NewStaticV4(*access, *secret, ""), Secure: u.Scheme == "https", Region: "us-east-1", BucketLookup: s3.BucketLookupPath, MaxRetries: 1})
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		err = api.MakeBucket(ctx, *bucket, s3.MakeBucketOptions{Region: "us-east-1"})
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			fail(fmt.Errorf("silo fixture bucket 创建失败: %w", err))
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("prepared bucket %s and isolated CI certificates in %s\n", *bucket, *out)
}

func authority() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "file-preview-image-e2e"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(6 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func leaf(ca *x509.Certificate, caKey *ecdsa.PrivateKey, host string) ([]byte, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(6 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	return der, key, nil
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		fail(err)
	}
	return n
}

func writePEM(path, kind string, der []byte) error {
	// 一次性 CI 材料：应用容器以非 root 用户读取同一挂载目录，这里不使用
	// 0600。这些密钥只存在于单次 CI 运行的隔离网络，不是任何环境的凭据。
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
