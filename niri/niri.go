// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package niri

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type (
	Event struct {
		WorkspacesChanged            WorkspacesChanged
		WorkspaceUrgencyChanged      WorkspaceUrgencyChanged
		WorkspaceActivated           WorkspaceActivated
		WorkspaceActiveWindowChanged WorkspaceActiveWindowChanged
		WindowsChanged               WindowsChanged
		WindowOpenedOrChanged        WindowOpenedOrChanged
		WindowClosed                 WindowClosed
		WindowFocusChanged           WindowFocusChanged
		WindowUrgencyChanged         WindowUrgencyChanged
		WindowLayoytsChanged         WindowLayoutsChanged
		KeyboardLayoutsChanged       KeyboardLayoutsChanged
		KeyboardLayoutSwitched       KeyboardLayoutSwitched
		OverviewOpenedOrClosed       OverviewOpenedOrClosed
		ConfigLoaded                 ConfigLoaded
	}

	WorkspacesChanged struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	WorkspaceUrgencyChanged struct {
		ID     uint64 `json:"id"`
		Urgent bool   `json:"urgent"`
	}
	WorkspaceActivated struct {
		ID      uint64 `json:"id"`
		Focused bool   `json:"focused"`
	}
	WorkspaceActiveWindowChanged struct {
		WorkspaceID    uint64 `json:"workspace_id"`
		ActiveWindowID uint64 `json:"active_window_id,omitempty"`
	}
	WindowsChanged struct {
		Windows []Window `json:"windows"`
	}
	WindowOpenedOrChanged struct {
		Window Window `json:"window"`
	}
	WindowClosed struct {
		ID uint64 `json:"id"`
	}
	WindowFocusChanged struct {
		ID uint64 `json:"id,omitempty"`
	}
	WindowUrgencyChanged struct {
		ID     uint64 `json:"id"`
		Urgent bool   `json:"urgent"`
	}
	WindowLayoutsChanged struct {
		Changes []WindowLayoutChange
	}
	WindowLayoutChange struct {
		ID           uint64
		WindowLayout WindowLayout
	}
	KeyboardLayoutsChanged struct {
		KeyboardLayouts KeyboardLayouts `json:"keyboard_layouts"`
	}
	KeyboardLayoutSwitched struct {
		Idx uint8 `json:"idx"`
	}
	OverviewOpenedOrClosed struct {
		IsOpen bool `json:"is_open"`
	}
	ConfigLoaded struct {
		Failed bool `json:"failed"`
	}

	WindowLayout struct {
		PosInScrollingLayout   [2]uint     `json:"pos_in_scrolling_layout,omitzero"`
		TileSize               [2]float64  `json:"tile_size"`
		WindowSize             [2]int32    `json:"window_size"`
		TilePosInWorkspaceView *[2]float64 `json:"tile_pos_in_workspace_view,omitempty"`
		WindowOffsetInTile     [2]float64  `json:"window_offset_in_tile"`
	}

	Window struct {
		ID          uint64       `json:"id"`
		Title       string       `json:"title,omitempty"`
		AppID       string       `json:"app_id,omitempty"`
		PID         int32        `json:"pid,omitempty"`
		WorkspaceID uint64       `json:"workspace_id,omitempty"`
		IsFocused   bool         `json:"is_focused"`
		IsFloating  bool         `json:"is_floating"`
		IsUrgent    bool         `json:"is_urgent"`
		Layout      WindowLayout `json:"layout"`
	}

	KeyboardLayouts struct {
		Names      []string `json:"names"`
		CurrentIdx uint8    `json:"current_idx"`
	}

	Workspace struct {
		ID             uint64 `json:"id"`
		Idx            uint8  `json:"idx"`
		Name           string `json:"name,omitempty"`
		Output         string `json:"output,omitempty"`
		IsUrgent       bool   `json:"is_urgent"`
		IsActive       bool   `json:"is_active"`
		IsFocused      bool   `json:"is_focused"`
		ActiveWindowID uint64 `json:"active_window_id,omitempty"`
	}

	Client struct {
		path string
		conn net.Conn
	}
)

func (wlc WindowLayoutChange) MarshalJSON() ([]byte, error) {
	// {"WindowLayoutsChanged":{"changes":[[60,{"pos_in_scrolling_layout":[3,1],"tile_size":[1908.0,1026.0],"window_size":[1908,1026],"tile_pos_in_workspace_view":null,"window_offset_in_tile":[0.0,0.0]}]]}}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[%d,", wlc.ID)
	err := json.NewEncoder(&buf).Encode(wlc.WindowLayout)
	buf.WriteByte(']')
	return buf.Bytes(), err
}

func (wlc *WindowLayoutChange) UnmarshalJSON(p []byte) error {
	if len(p) == 0 || bytes.Equal(p, []byte("null")) || bytes.Equal(p, []byte("[]")) {
		*wlc = WindowLayoutChange{}
		return nil
	}
	p = bytes.TrimPrefix(p, []byte("["))
	i := bytes.IndexByte(p, ',')
	var err error
	if wlc.ID, err = strconv.ParseUint(string(p[:i]), 10, 64); err != nil {
		return err
	}
	p = p[i+1:]
	i = bytes.IndexByte(p, '}')
	return json.Unmarshal(p[:i], &wlc.WindowLayout)
}

// New returns a Client configured to connect to $NIRI_SOCKET
func New(ctx context.Context) (Client, error) {
	c := Client{path: strings.TrimSpace(os.Getenv("NIRI_SOCKET"))}
	var err error
	if c.conn, err = (&net.Dialer{}).DialContext(ctx, "unix", c.path); err != nil {
		return c, err
	}
	return c, nil
}

func (c Client) Close() error {
	return c.conn.Close()
}

func (c Client) ListWindows(ctx context.Context) ([]Window, error) {
	p, err := c.do(ctx, "Windows")
	if err != nil {
		return nil, err
	}
	var ww []Window
	err = json.Unmarshal(p, &ww)
	return ww, err
}

func (c Client) Subscribe(ctx context.Context, f func(Event) error) error {
	slog.Debug("Subscribe", "path", c.path)
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("EventStream")); err != nil {
		return err
	}
	dec := json.NewDecoder(conn)
	var e Event
	for {
		if err := dec.Decode(&e); err != nil {
			return err
		}
		if err := f(e); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) do(ctx context.Context, command string) ([]byte, error) {
	dl, _ := ctx.Deadline()
	maxDl := time.Now().Add(3 * time.Second)
	if dl.After(maxDl) {
		dl = maxDl
	}
	c.conn.SetWriteDeadline(dl)
	c.conn.SetReadDeadline(dl)
	slog.Debug("do", "command", command)
	if _, err := c.conn.Write([]byte(strconv.Quote(command))); err != nil {
		slog.Error("Write", "error", err)
		return nil, err
	}
	slog.Debug("do written")
	var ans struct {
		Ok  json.RawMessage
		Err string
	}
	scanner := bufio.NewScanner(c.conn)
	if !scanner.Scan() {
		slog.Error("Scan", "error", scanner.Err())
		return nil, scanner.Err()
	}
	slog.Debug("scanned", "line", scanner.Text())
	if err := json.Unmarshal(scanner.Bytes(), &ans); err != nil {
		return nil, err
	}
	if ans.Err != "" {
		return nil, errors.New(ans.Err)
	}
	return ans.Ok, nil
}
