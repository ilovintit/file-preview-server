package main

import (
	"git.shw.top/shw-project/file-preview-server/internal/framework/cmd"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() { cmd.Main.Run(gctx.GetInitCtx()) }
