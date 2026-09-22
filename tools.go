package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerTools wires every userbot capability onto the MCP server.
func registerTools(s *server.MCPServer, t *tgClient) {
	s.AddTool(mcp.NewTool("get_me",
		mcp.WithDescription("Return the logged-in userbot account (id, name, username, phone)."),
	), t.hGetMe)

	s.AddTool(mcp.NewTool("resolve_peer",
		mcp.WithDescription("Resolve a @username, phone, or 'me' to a Telegram peer (id, type, name)."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
	), t.hResolve)

	s.AddTool(mcp.NewTool("list_dialogs",
		mcp.WithDescription("List recent chats/dialogs (users, groups, channels) with their peer ids."),
		mcp.WithNumber("limit", mcp.Description("Max dialogs (default 30)")),
	), t.hListDialogs)

	s.AddTool(mcp.NewTool("get_history",
		mcp.WithDescription("Fetch recent messages from a chat, newest first."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithNumber("limit", mcp.Description("Max messages (default 20)")),
		mcp.WithNumber("offset_id", mcp.Description("Only return messages older than this id (pagination)")),
	), t.hGetHistory)

	s.AddTool(mcp.NewTool("send_message",
		mcp.WithDescription("Send a text message to a chat as the userbot."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithString("text", mcp.Required(), mcp.Description("Message text")),
		mcp.WithNumber("reply_to", mcp.Description("Message id to reply to")),
	), t.hSendMessage)

	s.AddTool(mcp.NewTool("search_messages",
		mcp.WithDescription("Search messages containing a query within a specific chat."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search text")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 20)")),
	), t.hSearchMessages)

	s.AddTool(mcp.NewTool("search_global",
		mcp.WithDescription("Search messages across all chats."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search text")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 20)")),
	), t.hSearchGlobal)

	s.AddTool(mcp.NewTool("edit_message",
		mcp.WithDescription("Edit the text of a message you sent."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithNumber("message_id", mcp.Required(), mcp.Description("Message id to edit")),
		mcp.WithString("text", mcp.Required(), mcp.Description("New message text")),
	), t.hEditMessage)

	s.AddTool(mcp.NewTool("delete_messages",
		mcp.WithDescription("Delete messages from a chat."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithArray("message_ids", mcp.Required(), mcp.Items(map[string]any{"type": "number"}), mcp.Description("Message ids to delete")),
		mcp.WithBoolean("revoke", mcp.Description("Delete for everyone (default true). Channels always delete for everyone.")),
	), t.hDeleteMessages)

	s.AddTool(mcp.NewTool("forward_messages",
		mcp.WithDescription("Forward messages from one chat to another."),
		mcp.WithString("from_peer", mcp.Required(), mcp.Description("Source chat: "+peerDesc)),
		mcp.WithString("to_peer", mcp.Required(), mcp.Description("Destination chat: "+peerDesc)),
		mcp.WithArray("message_ids", mcp.Required(), mcp.Items(map[string]any{"type": "number"}), mcp.Description("Message ids to forward")),
	), t.hForwardMessages)

	s.AddTool(mcp.NewTool("mark_read",
		mcp.WithDescription("Mark all messages in a chat as read."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
	), t.hMarkRead)

	s.AddTool(mcp.NewTool("send_reaction",
		mcp.WithDescription("React to a message with an emoji. Empty emoji removes your reaction."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithNumber("message_id", mcp.Required(), mcp.Description("Message id to react to")),
		mcp.WithString("emoji", mcp.Description("Reaction emoji, e.g. 👍 (must be allowed in the chat)")),
	), t.hSendReaction)
}

const peerDesc = "@username, phone, t.me link, 'me', or a peer from list_dialogs (e.g. user:123, channel:456)"

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func (t *tgClient) hGetMe(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := t.ensureAuth(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	self, err := t.client.Self(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"id":       self.ID,
		"username": self.Username,
		"name":     fmt.Sprintf("%s %s", self.FirstName, self.LastName),
		"phone":    self.Phone,
		"bot":      self.Bot,
	})
}

func (t *tgClient) hResolve(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	kind := "user"
	switch p.(type) {
	case peers.Chat:
		kind = "chat"
	case peers.Channel:
		kind = "channel"
	}
	username, _ := p.Username()
	return jsonResult(map[string]any{
		"peer":     fmt.Sprintf("%s:%d", kind, p.ID()),
		"name":     p.VisibleName(),
		"username": username,
	})
}

func (t *tgClient) hSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b := &message.NewSender(t.api).To(p.InputPeer()).Builder
	if id := req.GetInt("reply_to", 0); id != 0 {
		b = b.Reply(id)
	}
	if _, err := b.Text(ctx, text); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("sent to %s", p.VisibleName())), nil
}

func (t *tgClient) hListDialogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := t.ensureAuth(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 30)
	resp, err := t.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      limit,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var dialogs []tg.DialogClass
	var users []tg.UserClass
	var chats []tg.ChatClass
	switch d := resp.(type) {
	case *tg.MessagesDialogs:
		dialogs, users, chats = d.Dialogs, d.Users, d.Chats
	case *tg.MessagesDialogsSlice:
		dialogs, users, chats = d.Dialogs, d.Users, d.Chats
	default:
		return mcp.NewToolResultError("unexpected dialogs response"), nil
	}
	names := nameIndex(users, chats)
	// Cache access hashes so "user:ID"/"channel:ID" peers resolve later.
	if err := t.peers.Apply(ctx, users, chats); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	out := make([]map[string]any, 0, len(dialogs))
	for _, dc := range dialogs {
		d, ok := dc.(*tg.Dialog)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"peer":         peerString(d.Peer),
			"name":         names[peerKey(d.Peer)],
			"unread_count": d.UnreadCount,
		})
	}
	return jsonResult(out)
}

func (t *tgClient) hGetHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 20)
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, err := t.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:     p.InputPeer(),
		OffsetID: req.GetInt("offset_id", 0),
		Limit:    limit,
	})
	return t.messagesResult(ctx, resp, err)
}

// messagesResult caches peers from a messages.* response and formats it.
func (t *tgClient) messagesResult(ctx context.Context, resp tg.MessagesMessagesClass, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	users, chats := entitiesOf(resp)
	if err := t.peers.Apply(ctx, users, chats); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(formatMessages(messagesOf(resp)))
}

func (t *tgClient) hSearchGlobal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := t.ensureAuth(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	q, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, err := t.api.MessagesSearchGlobal(ctx, &tg.MessagesSearchGlobalRequest{
		Q:          q,
		Filter:     &tg.InputMessagesFilterEmpty{},
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      req.GetInt("limit", 20),
	})
	return t.messagesResult(ctx, resp, err)
}

func (t *tgClient) hEditMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	p, id, err := t.peerAndMsg(ctx, req, "peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if _, err := message.NewSender(t.api).To(p.InputPeer()).Edit(id).Text(ctx, text); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("edited message %d", id)), nil
}

func (t *tgClient) hDeleteMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ids, err := req.RequireIntSlice("message_ids")
	if err != nil || len(ids) == 0 {
		return mcp.NewToolResultError("message_ids required"), nil
	}
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	sender := message.NewSender(t.api)
	_, isChannel := inputChannel(p)
	if req.GetBool("revoke", true) || isChannel {
		_, err = sender.To(p.InputPeer()).Revoke().Messages(ctx, ids...)
	} else {
		_, err = sender.Delete().Messages(ctx, ids...)
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("deleted %d message(s)", len(ids))), nil
}

func (t *tgClient) hForwardMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fromWho, err := req.RequireString("from_peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toWho, err := req.RequireString("to_peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ids, err := req.RequireIntSlice("message_ids")
	if err != nil || len(ids) == 0 {
		return mcp.NewToolResultError("message_ids required"), nil
	}
	from, err := t.resolve(ctx, fromWho)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	to, err := t.resolve(ctx, toWho)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	fb := message.NewSender(t.api).To(to.InputPeer()).ForwardIDs(from.InputPeer(), ids[0], ids[1:]...)
	if _, err := fb.Send(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("forwarded %d message(s) to %s", len(ids), to.VisibleName())), nil
}

func (t *tgClient) hMarkRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if ch, ok := inputChannel(p); ok {
		_, err = t.api.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{Channel: ch})
	} else {
		_, err = t.api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{Peer: p.InputPeer()})
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("marked %s as read", p.VisibleName())), nil
}

func (t *tgClient) hSendReaction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	p, id, err := t.peerAndMsg(ctx, req, "peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var reactions []tg.ReactionClass
	if e := req.GetString("emoji", ""); e != "" {
		reactions = append(reactions, &tg.ReactionEmoji{Emoticon: e})
	}
	if _, err := message.NewSender(t.api).To(p.InputPeer()).Reaction(ctx, id, reactions...); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("reacted to message %d", id)), nil
}

// peerAndMsg resolves the peer param plus a required message_id.
func (t *tgClient) peerAndMsg(ctx context.Context, req mcp.CallToolRequest, key string) (peers.Peer, int, error) {
	who, err := req.RequireString(key)
	if err != nil {
		return nil, 0, err
	}
	id := req.GetInt("message_id", 0)
	if id == 0 {
		return nil, 0, errors.New("message_id required")
	}
	p, err := t.resolve(ctx, who)
	return p, id, err
}

func (t *tgClient) hSearchMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	q, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 20)
	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, err := t.api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
		Peer:   p.InputPeer(),
		Q:      q,
		Filter: &tg.InputMessagesFilterEmpty{},
		Limit:  limit,
	})
	return t.messagesResult(ctx, resp, err)
}
