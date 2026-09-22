package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerAuthTools exposes the interactive login flow as MCP tools so a client
// can log the userbot in without a separate terminal session.
//
// Flow: auth_status → auth_send_code(phone) → auth_sign_in(code)
//
//	→ (if 2FA) auth_password(password).
func registerAuthTools(s *server.MCPServer, t *tgClient) {
	s.AddTool(mcp.NewTool("auth_status",
		mcp.WithDescription("Report whether the userbot is logged in."),
	), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if t.isAuthorized() {
			return mcp.NewToolResultText("authorized"), nil
		}
		return mcp.NewToolResultText("not authorized — call auth_send_code(phone) to begin login"), nil
	})

	s.AddTool(mcp.NewTool("auth_send_code",
		mcp.WithDescription("Start login: send a login code to the given phone via Telegram."),
		mcp.WithString("phone", mcp.Required(), mcp.Description("International phone, e.g. +15551234567")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		phone, err := req.RequireString("phone")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := t.sendCode(ctx, phone); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("code sent — call auth_sign_in with the code from Telegram"), nil
	})

	s.AddTool(mcp.NewTool("auth_sign_in",
		mcp.WithDescription("Submit the login code Telegram sent. If 2FA is enabled, follow with auth_password."),
		mcp.WithString("code", mcp.Required(), mcp.Description("Login code from Telegram")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code, err := req.RequireString("code")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		needPass, err := t.signIn(ctx, code)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if needPass {
			return mcp.NewToolResultText("2FA enabled — call auth_password with your cloud password"), nil
		}
		return mcp.NewToolResultText("logged in"), nil
	})

	s.AddTool(mcp.NewTool("auth_password",
		mcp.WithDescription("Complete login with your Telegram 2FA (cloud) password."),
		mcp.WithString("password", mcp.Required(), mcp.Description("Telegram 2FA password")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pw, err := req.RequireString("password")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := t.submitPassword(ctx, pw); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("logged in"), nil
	})
}
