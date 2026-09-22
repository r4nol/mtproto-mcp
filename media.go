package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMediaTools(s *server.MCPServer, t *tgClient) {
	s.AddTool(mcp.NewTool("send_file",
		mcp.WithDescription("Upload a local file and send it to a chat as the userbot."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the local file")),
		mcp.WithString("caption", mcp.Description("Optional caption text")),
		mcp.WithBoolean("as_photo", mcp.Description("Send as a photo (compressed) instead of a document. Default false.")),
	), t.hSendFile)

	s.AddTool(mcp.NewTool("download_media",
		mcp.WithDescription("Download the media (photo/document) of a message to a local directory."),
		mcp.WithString("peer", mcp.Required(), mcp.Description(peerDesc)),
		mcp.WithNumber("message_id", mcp.Required(), mcp.Description("Message id containing the media")),
		mcp.WithString("out_dir", mcp.Description("Directory to save into (default current dir)")),
	), t.hDownloadMedia)
}

func (t *tgClient) hSendFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	caption := req.GetString("caption", "")
	asPhoto := req.GetBool("as_photo", false)

	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	u := uploader.NewUploader(t.api)
	file, err := u.FromPath(ctx, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upload failed: %v", err)), nil
	}

	var cap []message.StyledTextOption
	if caption != "" {
		cap = append(cap, styling.Plain(caption))
	}

	var media message.MediaOption
	if asPhoto {
		media = message.UploadedPhoto(file, cap...)
	} else {
		media = message.UploadedDocument(file, cap...).Filename(filepath.Base(path))
	}

	sender := message.NewSender(t.api).WithUploader(u)
	if _, err := sender.To(p.InputPeer()).Media(ctx, media); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("sent %s to %s", filepath.Base(path), p.VisibleName())), nil
}

func (t *tgClient) hDownloadMedia(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	who, err := req.RequireString("peer")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msgID := req.GetInt("message_id", 0)
	if msgID == 0 {
		return mcp.NewToolResultError("message_id required"), nil
	}
	outDir := req.GetString("out_dir", ".")

	p, err := t.resolve(ctx, who)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	m, err := t.getMessage(ctx, p, msgID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	loc, name, err := mediaLocation(m)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	outPath := filepath.Join(outDir, name)

	d := downloader.NewDownloader()
	if _, err := d.Download(t.api, loc).ToPath(ctx, outPath); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("saved to %s", outPath)), nil
}

// getMessage fetches a single message by id, handling channel vs non-channel peers.
func (t *tgClient) getMessage(ctx context.Context, p peers.Peer, id int) (*tg.Message, error) {
	ids := []tg.InputMessageClass{&tg.InputMessageID{ID: id}}
	var resp tg.MessagesMessagesClass
	var err error
	if ch, ok := inputChannel(p); ok {
		resp, err = t.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: ch,
			ID:      ids,
		})
	} else {
		resp, err = t.api.MessagesGetMessages(ctx, ids)
	}
	if err != nil {
		return nil, err
	}
	for _, mc := range messagesOf(resp) {
		if m, ok := mc.(*tg.Message); ok && m.ID == id {
			return m, nil
		}
	}
	return nil, fmt.Errorf("message %d not found or has no accessible content", id)
}

// mediaLocation returns the download location + suggested filename for a message's media.
func mediaLocation(m *tg.Message) (tg.InputFileLocationClass, string, error) {
	switch media := m.Media.(type) {
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.AsNotEmpty()
		if !ok {
			return nil, "", fmt.Errorf("document unavailable")
		}
		name := fmt.Sprintf("msg_%d.bin", m.ID)
		for _, attr := range doc.Attributes {
			// FileName is sender-controlled: keep only the base name.
			if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
				if b := filepath.Base(fn.FileName); b != "." && b != ".." && b != string(filepath.Separator) {
					name = b
				}
			}
		}
		return doc.AsInputDocumentFileLocation(), name, nil

	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.AsNotEmpty()
		if !ok {
			return nil, "", fmt.Errorf("photo unavailable")
		}
		thumb := largestPhotoSize(photo.Sizes)
		if thumb == "" {
			return nil, "", fmt.Errorf("no downloadable photo size")
		}
		return &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumb,
		}, fmt.Sprintf("msg_%d.jpg", m.ID), nil

	default:
		return nil, "", fmt.Errorf("message %d has no downloadable media", m.ID)
	}
}

// largestPhotoSize picks the biggest concrete photo size type.
func largestPhotoSize(sizes []tg.PhotoSizeClass) string {
	best := ""
	max := 0
	for _, s := range sizes {
		if ps, ok := s.(*tg.PhotoSize); ok && ps.Size > max {
			max = ps.Size
			best = ps.Type
		}
	}
	if best == "" {
		// fall back to the last size type available (e.g. progressive)
		for _, s := range sizes {
			if ps, ok := s.(*tg.PhotoSizeProgressive); ok {
				best = ps.Type
			}
		}
	}
	return best
}
