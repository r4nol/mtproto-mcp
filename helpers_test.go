package main

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestMediaLocationSanitizesFilename(t *testing.T) {
	for in, want := range map[string]string{
		"report.pdf":            "report.pdf",
		"../../.ssh/authorized": "authorized",
		"/etc/passwd":           "passwd",
		"..":                    "msg_7.bin",
	} {
		m := &tg.Message{ID: 7, Media: &tg.MessageMediaDocument{
			Document: &tg.Document{Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: in},
			}},
		}}
		_, got, err := mediaLocation(m)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestPeerString(t *testing.T) {
	cases := map[tg.PeerClass]string{
		&tg.PeerUser{UserID: 1}:       "user:1",
		&tg.PeerChat{ChatID: 2}:       "chat:2",
		&tg.PeerChannel{ChannelID: 3}: "channel:3",
	}
	for p, want := range cases {
		if got := peerString(p); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
