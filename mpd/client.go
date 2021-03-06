// Copyright 2009 The GoMPD Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

// This package provides the client side interface to MPD (Music Player Daemon).
// The protocol reference can be found at http://www.musicpd.org/doc/protocol/index.html
package mpd

import (
	"net/textproto"
	"os"
	"strconv"
	"strings"
)

type Client struct {
	text *textproto.Conn
}

type Attrs map[string]string

// Dial connects to MPD listening on address addr (e.g. "127.0.0.1:6600")
// on network network (e.g. "tcp").
func Dial(network, addr string) (c *Client, err os.Error) {
	text, err := textproto.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	line, err := text.ReadLine()
	if err != nil {
		return nil, err
	}
	if line[0:6] != "OK MPD" {
		return nil, textproto.ProtocolError("no greeting")
	}
	return &Client{text: text}, nil
}

// Close terminates the connection with MPD.
func (c *Client) Close() (err os.Error) {
	if c.text != nil {
		c.text.PrintfLine("close")
		err = c.text.Close()
		c.text = nil
	}
	return
}

// Ping sends a no-op message to MPD. It's useful for keeping the connection alive.
func (c *Client) Ping() os.Error {
	return c.okCmd("ping")
}

func (c *Client) readPlaylist() (pls []Attrs, err os.Error) {
	pls = []Attrs{}

	for {
		line, err := c.text.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "OK" {
			break
		}
		if strings.HasPrefix(line, "file:") { // new song entry begins
			pls = append(pls, Attrs{})
		}
		if len(pls) == 0 {
			return nil, textproto.ProtocolError("unexpected: " + line)
		}
		z := strings.Index(line, ": ")
		if z < 0 {
			return nil, textproto.ProtocolError("can't parse line: " + line)
		}
		key := line[0:z]
		pls[len(pls)-1][key] = line[z+2:]
	}
	return pls, nil
}

func (c *Client) readAttrs() (attrs Attrs, err os.Error) {
	attrs = make(Attrs)
	for {
		line, err := c.text.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "OK" {
			break
		}
		z := strings.Index(line, ": ")
		if z < 0 {
			return nil, textproto.ProtocolError("can't parse line: " + line)
		}
		key := line[0:z]
		attrs[key] = line[z+2:]
	}
	return
}

// CurrentSong returns information about the current song in the playlist.
func (c *Client) CurrentSong() (Attrs, os.Error) {
	id, err := c.text.Cmd("currentsong")
	if err != nil {
		return nil, err
	}
	c.text.StartResponse(id)
	defer c.text.EndResponse(id)
	return c.readAttrs()
}

// Status returns information about the current status of MPD.
func (c *Client) Status() (Attrs, os.Error) {
	id, err := c.text.Cmd("status")
	if err != nil {
		return nil, err
	}
	c.text.StartResponse(id)
	defer c.text.EndResponse(id)
	return c.readAttrs()
}

func (c *Client) readOKLine() (err os.Error) {
	line, err := c.text.ReadLine()
	if err != nil {
		return
	}
	if line == "OK" {
		return nil
	}
	return textproto.ProtocolError("unexpected response: " + line)
}

func (c *Client) okCmd(format string, args ...interface{}) os.Error {
	id, err := c.text.Cmd(format, args...)
	if err != nil {
		return err
	}
	c.text.StartResponse(id)
	defer c.text.EndResponse(id)
	return c.readOKLine()
}

//
// Playback control
//

// Next plays next song in the playlist.
func (c *Client) Next() os.Error {
	return c.okCmd("next")
}

// Pause pauses playback if pause is true; resumes playback otherwise.
func (c *Client) Pause(pause bool) os.Error {
	if pause {
		return c.okCmd("pause 1")
	}
	return c.okCmd("pause 0")
}

// Play starts playing the song at playlist position pos. If pos is negative,
// start playing at the current position in the playlist.
func (c *Client) Play(pos int) os.Error {
	if pos < 0 {
		c.okCmd("play")
	}
	return c.okCmd("play %d", pos)
}

// PlayId plays the song identified by id. If id is negative, start playing
// at the currect position in playlist.
func (c *Client) PlayId(id int) os.Error {
	if id < 0 {
		return c.okCmd("playid")
	}
	return c.okCmd("playid %d", id)
}

// Previous plays previous song in the playlist.
func (c *Client) Previous() os.Error {
	return c.okCmd("next")
}

// Seek seeks to the position time (in seconds) of the song at playlist position pos.
func (c *Client) Seek(pos, time int) os.Error {
	return c.okCmd("seek %d %d", pos, time)
}

// SeekId is identical to Seek except the song is identified by it's id
// (not position in playlist).
func (c *Client) SeekId(id, time int) os.Error {
	return c.okCmd("seekid %d %d", id, time)
}

// Stop stops playback.
func (c *Client) Stop() os.Error {
	return c.okCmd("stop")
}

//
// Playlist related functions
//

// PlaylistInfo returns attributes for songs in the current playlist. If
// both start and end are negative, it does this for all songs in
// playlist. If end is negative but start is positive, it does it for the
// song at position start. If both start and end are positive, it does it
// for positions in range [start, end).
func (c *Client) PlaylistInfo(start, end int) (pls []Attrs, err os.Error) {
	if start < 0 && end >= 0 {
		return nil, os.NewError("negative start index")
	}
	if start >= 0 && end < 0 {
		id, err := c.text.Cmd("playlistinfo %d", start)
		if err != nil {
			return nil, err
		}
		c.text.StartResponse(id)
		defer c.text.EndResponse(id)
		return c.readPlaylist()
	}
	id, err := c.text.Cmd("playlistinfo")
	if err != nil {
		return nil, err
	}
	c.text.StartResponse(id)
	defer c.text.EndResponse(id)
	pls, err = c.readPlaylist()
	if err != nil || start < 0 || end < 0 {
		return
	}
	return pls[start:end], nil
}

// Delete deletes songs from playlist. If both start and end are positive,
// it deletes those at positions in range [start, end). If end is negative,
// it deletes the song at position start.
func (c *Client) Delete(start, end int) os.Error {
	if start < 0 {
		return os.NewError("negative start index")
	}
	if end < 0 {
		return c.okCmd("delete %d", start)
	}
	return c.okCmd("delete %d %d", start, end)
}

// DeleteId deletes the song identified by id.
func (c *Client) DeleteId(id int) os.Error {
	return c.okCmd("deleteid %d", id)
}

// Add adds the file/directory uri to playlist. Directories add recursively.
func (c *Client) Add(uri string) os.Error {
	return c.okCmd("add %q", uri)
}

// AddId adds the file/directory uri to playlist and returns the identity
// id of the song added. If pos is positive, the song is added to position
// pos.
func (c *Client) AddId(uri string, pos int) (int, os.Error) {
	var id uint
	var err os.Error
	if pos >= 0 {
		id, err = c.text.Cmd("addid %q %d", uri, pos)
	}
	id, err = c.text.Cmd("addid %q", uri)
	if err != nil {
		return -1, err
	}

	c.text.StartResponse(id)
	defer c.text.EndResponse(id)

	attrs, err := c.readAttrs()
	if err != nil {
		return -1, err
	}
	tok, ok := attrs["Id"]
	if !ok {
		return -1, textproto.ProtocolError("addid did not return Id")
	}
	return strconv.Atoi(tok)
}

// Clear clears the current playlist.
func (c *Client) Clear() os.Error {
	return c.okCmd("clear")
}
