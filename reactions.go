package mtpwrap

import (
	"context"
	"errors"

	"github.com/gotd/td/tg"
)

var ErrNoChannelReactions = errors.New("no channel reactions")

// ChannelReactions returns available channel reactions.
func (cl *Client) ChannelReactions(ctx context.Context, channel tg.InputChannelClass) (tg.ChatReactionsClass, error) {
	mcf, err := cl.cl.API().ChannelsGetFullChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	ar, ok := mcf.FullChat.GetAvailableReactions()
	if !ok {
		return nil, ErrNoChannelReactions
	}
	return ar, nil
}
