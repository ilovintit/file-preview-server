package cmd

import (
	"context"
	preview "git.shw.top/shw-project/file-preview-server/internal/module/preview"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
)

var Main = gcmd.Command{Name: "file-preview-server", Usage: "file-preview-server server", Func: func(ctx context.Context, _ *gcmd.Parser) error { return run(ctx) }}

func init() {
	_ = Main.AddCommand(&gcmd.Command{Name: "server", Func: func(ctx context.Context, _ *gcmd.Parser) error { return run(ctx) }})
}

func run(ctx context.Context) error {
	m, err := preview.New(infrastructure.Config{}, nil)
	if err != nil {
		return err
	}
	defer m.Close(ctx)
	s := ghttp.GetServer("preview")
	s.SetAddr(":9501")
	s.SetGraceful(false)
	s.SetAccessLogEnabled(false)
	s.SetErrorLogEnabled(false)
	s.SetDumpRouterMap(false)
	s.SetHandler(m.Handler().ServeHTTP)
	s.Run()
	return nil
}
