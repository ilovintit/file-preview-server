package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	preview "git.shw.top/shw-project/file-preview-server/internal/module/preview"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"os"
	"time"
)

var Main = gcmd.Command{Name: "file-preview-server", Usage: "file-preview-server server", Func: func(ctx context.Context, _ *gcmd.Parser) error { return run(ctx) }}

func init() {
	_ = Main.AddCommand(&gcmd.Command{Name: "server", Func: func(ctx context.Context, _ *gcmd.Parser) error { return run(ctx) }})
}

func run(ctx context.Context) error {
	cfg, err := infrastructure.LoadConfig()
	if err != nil {
		return err
	}
	m, err := preview.New(cfg, nil)
	if err != nil {
		return err
	}
	defer m.Close(ctx)
	s := ghttp.GetServer("preview")
	s.SetAddr(cfg.Address)
	s.SetGraceful(false)
	s.SetAccessLogEnabled(false)
	s.SetErrorLogEnabled(false)
	s.SetDumpRouterMap(false)
	s.SetReadTimeout(5 * time.Second)
	s.SetWriteTimeout(15 * time.Second)
	s.SetIdleTimeout(30 * time.Second)
	s.SetMaxHeaderBytes(16 * 1024)
	if cfg.TLSCert != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return gerror.New("invalid server TLS files")
		}
		config := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
		if cfg.TLSClientCA != "" {
			pem, err := os.ReadFile(cfg.TLSClientCA)
			if err != nil {
				return gerror.New("invalid TLS client CA file")
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return gerror.New("invalid TLS client CA")
			}
			config.ClientCAs = pool
			config.ClientAuth = tls.RequireAndVerifyClientCert
		}
		s.SetTLSConfig(config)
		s.SetHTTPSAddr(cfg.Address)
		s.SetAddr("")
	}
	s.SetHandler(m.Handler().ServeHTTP)
	s.Run()
	return nil
}
