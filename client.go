package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
)

// tgClient keeps a gotd MTProto connection alive for the lifetime of the MCP
// server. gotd's Run blocks, so we drive it from a goroutine and hand the
// authenticated API out through ready/err channels.
type tgClient struct {
	client *telegram.Client
	api    *tg.Client
	peers  *peers.Manager
	stop   context.CancelFunc

	mu         sync.Mutex
	authorized bool
	phone      string // pending login
	codeHash   string // pending login
}

func newTGClient(cfg config) *tgClient {
	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: cfg.SessionPath},
	})
	return &tgClient{client: client}
}

// start connects and blocks until authenticated (or fails). The connection
// stays open in a background goroutine until Stop is called.
func (t *tgClient) start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	t.stop = cancel

	ready := make(chan error, 1)
	go func() {
		ready <- t.client.Run(runCtx, func(ctx context.Context) error {
			t.api = t.client.API()
			t.peers = peers.Options{}.Build(t.api)
			status, err := t.client.Auth().Status(ctx)
			if err != nil {
				return err
			}
			t.setAuthorized(status.Authorized)
			ready <- nil // signal connected (authed or not)
			<-ctx.Done() // hold the connection open
			return ctx.Err()
		})
	}()

	// First value on the channel is either the connected signal (nil) or a fatal error.
	if err := <-ready; err != nil {
		cancel()
		return err
	}
	return nil
}

func (t *tgClient) setAuthorized(v bool) {
	t.mu.Lock()
	t.authorized = v
	t.mu.Unlock()
}

func (t *tgClient) isAuthorized() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.authorized
}

// ensureAuth guards tools that require a logged-in account.
func (t *tgClient) ensureAuth() error {
	if !t.isAuthorized() {
		return errors.New("not logged in — call auth_send_code then auth_sign_in (see auth_status)")
	}
	return nil
}

// sendCode requests a login code for phone; stores the phone code hash.
func (t *tgClient) sendCode(ctx context.Context, phone string) error {
	sent, err := t.client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
	if err != nil {
		return err
	}
	sc, ok := sent.(*tg.AuthSentCode)
	if !ok {
		return fmt.Errorf("unexpected sent-code response %T", sent)
	}
	t.mu.Lock()
	t.phone = phone
	t.codeHash = sc.PhoneCodeHash
	t.mu.Unlock()
	return nil
}

// signIn submits the login code. Returns needPassword=true if 2FA is required.
func (t *tgClient) signIn(ctx context.Context, code string) (needPassword bool, err error) {
	t.mu.Lock()
	phone, hash := t.phone, t.codeHash
	t.mu.Unlock()
	if phone == "" || hash == "" {
		return false, errors.New("no pending login — call auth_send_code first")
	}
	if _, err := t.client.Auth().SignIn(ctx, phone, code, hash); err != nil {
		if errors.Is(err, auth.ErrPasswordAuthNeeded) {
			return true, nil
		}
		return false, err
	}
	t.setAuthorized(true)
	return false, nil
}

// submitPassword completes 2FA login.
func (t *tgClient) submitPassword(ctx context.Context, password string) error {
	if _, err := t.client.Auth().Password(ctx, password); err != nil {
		return err
	}
	t.setAuthorized(true)
	return nil
}

func (t *tgClient) Stop() {
	if t.stop != nil {
		t.stop()
	}
}

// resolve turns "me", a @username, phone, t.me link, or a "user:ID" /
// "chat:ID" / "channel:ID" string (as returned by list_dialogs) into a peer.
func (t *tgClient) resolve(ctx context.Context, who string) (peers.Peer, error) {
	if err := t.ensureAuth(); err != nil {
		return nil, err
	}
	if who == "me" || who == "self" {
		u, err := t.peers.Self(ctx)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	if kind, idStr, ok := strings.Cut(who, ":"); ok {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			switch kind {
			case "user":
				return t.peers.ResolveUserID(ctx, id)
			case "chat":
				return t.peers.ResolveChatID(ctx, id)
			case "channel":
				return t.peers.ResolveChannelID(ctx, id)
			}
		}
	}
	return t.peers.Resolve(ctx, who)
}

// inputChannel returns the channel handle when p is a channel/supergroup;
// several MTProto methods have separate channel variants.
func inputChannel(p peers.Peer) (*tg.InputChannel, bool) {
	ip, ok := p.InputPeer().(*tg.InputPeerChannel)
	if !ok {
		return nil, false
	}
	return &tg.InputChannel{ChannelID: ip.ChannelID, AccessHash: ip.AccessHash}, true
}
