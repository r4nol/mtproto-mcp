package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "login" {
		if err := runLogin(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "login failed:", err)
			os.Exit(1)
		}
		return
	}

	if err := serve(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}

func serve(cfg config) error {
	tc := newTGClient(cfg)
	if err := tc.start(context.Background()); err != nil {
		return err
	}
	defer tc.Stop()

	s := server.NewMCPServer("mtproto-mcp", version,
		server.WithToolCapabilities(true))
	registerAuthTools(s, tc)
	registerTools(s, tc)
	registerMediaTools(s, tc)

	return server.ServeStdio(s)
}
