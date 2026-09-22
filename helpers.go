package main

import (
	"fmt"
	"time"

	"github.com/gotd/td/tg"
)

// messagesOf extracts the message slice from any messages.* response type.
func messagesOf(resp tg.MessagesMessagesClass) []tg.MessageClass {
	switch m := resp.(type) {
	case *tg.MessagesMessages:
		return m.Messages
	case *tg.MessagesMessagesSlice:
		return m.Messages
	case *tg.MessagesChannelMessages:
		return m.Messages
	default:
		return nil
	}
}

// entitiesOf extracts the users/chats bundled with any messages.* response.
func entitiesOf(resp tg.MessagesMessagesClass) ([]tg.UserClass, []tg.ChatClass) {
	switch m := resp.(type) {
	case *tg.MessagesMessages:
		return m.Users, m.Chats
	case *tg.MessagesMessagesSlice:
		return m.Users, m.Chats
	case *tg.MessagesChannelMessages:
		return m.Users, m.Chats
	default:
		return nil, nil
	}
}

func formatMessages(msgs []tg.MessageClass) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, mc := range msgs {
		m, ok := mc.(*tg.Message)
		if !ok {
			continue // service messages, etc.
		}
		row := map[string]any{
			"id":   m.ID,
			"date": time.Unix(int64(m.Date), 0).UTC().Format(time.RFC3339),
			"out":  m.Out,
			"from": fromID(m),
			"chat": peerString(m.PeerID),
			"text": m.Message,
		}
		if r, ok := m.ReplyTo.(*tg.MessageReplyHeader); ok && r.ReplyToMsgID != 0 {
			row["reply_to"] = r.ReplyToMsgID
		}
		if kind := mediaKind(m.Media); kind != "" {
			row["media"] = kind
		}
		out = append(out, row)
	}
	return out
}

func mediaKind(m tg.MessageMediaClass) string {
	switch m.(type) {
	case nil, *tg.MessageMediaEmpty:
		return ""
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	case *tg.MessageMediaWebPage:
		return "webpage"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive, *tg.MessageMediaVenue:
		return "location"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaPoll:
		return "poll"
	default:
		return "other"
	}
}

func fromID(m *tg.Message) int64 {
	switch f := m.FromID.(type) {
	case *tg.PeerUser:
		return f.UserID
	case *tg.PeerChat:
		return f.ChatID
	case *tg.PeerChannel:
		return f.ChannelID
	default:
		return 0
	}
}

// peerKey / peerString give a stable identity for a peer.
func peerKey(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerUser:
		return v.UserID
	case *tg.PeerChat:
		return v.ChatID
	case *tg.PeerChannel:
		return v.ChannelID
	default:
		return 0
	}
}

func peerString(p tg.PeerClass) string {
	switch v := p.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("user:%d", v.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("chat:%d", v.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("channel:%d", v.ChannelID)
	default:
		return "unknown"
	}
}

// nameIndex maps peer id -> display name from the users/chats returned alongside dialogs.
func nameIndex(users []tg.UserClass, chats []tg.ChatClass) map[int64]string {
	m := make(map[int64]string, len(users)+len(chats))
	for _, uc := range users {
		if u, ok := uc.(*tg.User); ok {
			name := u.FirstName
			if u.LastName != "" {
				name += " " + u.LastName
			}
			if name == "" {
				name = u.Username
			}
			m[u.ID] = name
		}
	}
	for _, cc := range chats {
		switch c := cc.(type) {
		case *tg.Chat:
			m[c.ID] = c.Title
		case *tg.Channel:
			m[c.ID] = c.Title
		}
	}
	return m
}
