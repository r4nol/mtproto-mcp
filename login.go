package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

// termAuth drives the interactive phone-code (and 2FA password) login flow.
type termAuth struct {
	phone string
	r     *bufio.Reader
}

func (a termAuth) Phone(_ context.Context) (string, error) {
	if a.phone != "" {
		return a.phone, nil
	}
	fmt.Print("Phone (international, e.g. +15551234567): ")
	line, err := a.r.ReadString('\n')
	return strings.TrimSpace(line), err
}

func (a termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Code (sent via Telegram): ")
	line, err := a.r.ReadString('\n')
	return strings.TrimSpace(line), err
}

func (a termAuth) Password(_ context.Context) (string, error) {
	fmt.Print("2FA password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	return strings.TrimSpace(string(b)), err
}

func (a termAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	fmt.Println(tos.Text)
	return nil
}

func (a termAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up not supported; use an existing account")
}

func runLogin(cfg config) error {
	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: cfg.SessionPath},
	})
	return client.Run(context.Background(), func(ctx context.Context) error {
		flow := auth.NewFlow(
			termAuth{phone: cfg.Phone, r: bufio.NewReader(os.Stdin)},
			auth.SendCodeOptions{},
		)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}
		self, err := client.Self(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("Logged in as %s (id=%d). Session saved to %s\n",
			self.Username, self.ID, cfg.SessionPath)
		return nil
	})
}
