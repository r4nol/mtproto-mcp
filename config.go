package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// config holds the userbot credentials + session location.
// API ID/hash come from https://my.telegram.org (userbot = MTProto user account).
type config struct {
	APIID       int
	APIHash     string
	Phone       string
	SessionPath string
}

func loadConfig() (config, error) {
	var c config

	idStr := os.Getenv("TG_API_ID")
	if idStr == "" {
		return c, fmt.Errorf("TG_API_ID not set (get it from https://my.telegram.org)")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c, fmt.Errorf("TG_API_ID invalid: %w", err)
	}
	c.APIID = id

	c.APIHash = os.Getenv("TG_API_HASH")
	if c.APIHash == "" {
		return c, fmt.Errorf("TG_API_HASH not set (get it from https://my.telegram.org)")
	}

	c.Phone = os.Getenv("TG_PHONE") // only needed for `login`

	c.SessionPath = os.Getenv("TG_SESSION")
	if c.SessionPath == "" {
		home, _ := os.UserHomeDir()
		c.SessionPath = filepath.Join(home, ".go-tg-mcp.session.json")
	}
	return c, nil
}
