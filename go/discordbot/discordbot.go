// Package discordbot provides helper functionality around discordgo, useful for
// creating bots.
package discordbot

import (
	"context"
	"fmt"
	"sync"

	discord "github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

// A Bot is a Discord Bot.
type Bot struct {
	*discord.Session

	errMu      sync.RWMutex
	errHandler func(err error)
}

// Connect connects to the Discord API as a bot, opens a session, and blocks
// until a Ready event is received or the Context expires.
func Connect(ctx context.Context, token string) (*Bot, *discord.Ready, error) {
	sess, err := discord.New(fmt.Sprintf("Bot %s", token))
	if err != nil {
		return nil, nil, fmt.Errorf("discordgo.New([Bot token]): %v", err)
	}

	// discordgo is built around callbacks, which are not common in idiomatic
	// Go. If we had instead accepted a callback argument here, it would force
	// the call site to read non-linearly.
	ready := make(chan *discord.Ready)
	sess.AddHandlerOnce(func(_ *discord.Session, r *discord.Ready) {
		defer close(ready)
		select {
		case <-ctx.Done():
		case ready <- r:
		}
	})

	if err := sess.Open(); err != nil {
		return nil, nil, fmt.Errorf("%T.Open(): %v", sess, err)
	}

	select {
	case <-ctx.Done():
		// TODO(arran) in this scenario we leak the ready channel because it's
		// never closed. Although sess.AddHandler*() returns a removal function,
		// this doesn't guarantee that there's no race condition between removal
		// and a Ready event, so closing the channel here might cause a panic.
		sess.Close()
		return nil, nil, ctx.Err()
	case r := <-ready:
		return &Bot{Session: sess}, r, nil
	}
}

// SetErrHandler sets the function that is called when an error occurs while
// processing a callback.
func (b *Bot) SetErrHandler(h func(error)) {
	b.errMu.Lock()
	b.errHandler = h
	b.errMu.Unlock()
}

// handleErr logs the error at level ERROR and, if a non-nil handler is set,
// propagates the error.
func (b *Bot) handleErr(err error) {
	if err == nil {
		return
	}

	glog.ErrorDepth(1, err)

	b.errMu.RLock()
	defer b.errMu.RUnlock()
	if b.errHandler != nil {
		b.errHandler(err)
	}
}
