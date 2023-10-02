package discordbot

import (
	"fmt"

	discord "github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

// A Reaction contains information about a reaction by a Giver on a message
// created by the Receiver of the reaction.
type Reaction struct {
	*discord.MessageReaction
	Giver, Receiver *discord.User
}

// ReactionAdded embeds a Reaction to signal that it was added.
type ReactionAdded struct {
	Reaction
}

// ReactionRemoved embeds a Reaction to signal that it was removed.
type ReactionRemoved struct {
	Reaction
}

// OnEmojiAdded calls cb when the specified emoji is added on any message. The
// emoji is identified by the value returned by discordgo.Eomji.APIName(); an
// empty string matches all emoji.
func (b *Bot) OnEmojiAdded(name string, cb func(*discord.Session, *ReactionAdded) error) {
	b.Session.AddHandler(func(s *discord.Session, reaction *discord.MessageReactionAdd) {
		if !shouldProcessEmoji(reaction.Emoji, name) {
			if glog.V(1) {
				glog.Infof("Ignoring emoji %q added", reaction.Emoji.APIName())
			}
			return
		}

		msg, err := s.ChannelMessage(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			b.handleErr(fmt.Errorf("%T.ChannelMessage(%q, %q): %w", s, reaction.ChannelID, reaction.MessageID, err))
			return
		}

		b.handleErr(cb(
			b.Session,
			&ReactionAdded{Reaction{
				Giver:           reaction.Member.User,
				Receiver:        msg.Author,
				MessageReaction: reaction.MessageReaction,
			}},
		))
	})
}

// OnEmojiRemoved calls cb when the specified emoji is removed from any message.
// See OnEmojiAdded() for details on emoji names.
func (b *Bot) OnEmojiRemoved(name string, cb func(*discord.Session, *ReactionRemoved) error) {
	b.Session.AddHandler(func(s *discord.Session, reaction *discord.MessageReactionRemove) {
		if !shouldProcessEmoji(reaction.Emoji, name) {
			if glog.V(1) {
				glog.Infof("Ignoring emoji %q removed", reaction.Emoji.APIName())
			}
			return
		}

		msg, err := s.ChannelMessage(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			b.handleErr(fmt.Errorf("%T.ChannelMessage(%q, %q): %v", s, reaction.ChannelID, reaction.MessageID, err))
			return
		}

		u, err := s.User(reaction.UserID)
		if err != nil {
			b.handleErr(fmt.Errorf("%T.User(%q): %v", s, reaction.UserID, err))
			return
		}

		b.handleErr(cb(b.Session,
			&ReactionRemoved{Reaction{
				Giver:           u,
				Receiver:        msg.Author,
				MessageReaction: reaction.MessageReaction,
			}},
		))
	})
}

// shouldProcessEmoji returns true if name is an empty string or e.APIName().
func shouldProcessEmoji(e discord.Emoji, name string) bool {
	return name == "" || name == e.APIName()
}
