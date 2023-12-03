package mtpwrap

import (
	"context"
	"errors"
	"runtime/trace"

	"github.com/gotd/contrib/storage"

	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
)

// DialogType represents the dialog type. See [API Reference].
//
// [API Reference]: https://core.telegram.org/api/channel
//
//go:generate stringer -type=DialogType -trimprefix=D
type DialogType int

const (
	DUnknown DialogType = iota
	DGroup
	DMegagroup
	DGigagroup
	DChannel
)

// DlgType returns the type of the underlying dialog.
func DlgType(e Entity) DialogType {
	// we examine the underlying type and flags on it to figure out the type.
	switch tgt := e.(type) {
	case *tg.Channel:
		switch {
		case tgt.Broadcast:
			return DChannel
		case tgt.Megagroup:
			return DMegagroup
		case tgt.Gigagroup:
			return DGigagroup
		}
	case *tg.Chat:
		return DGroup
	}
	return DUnknown
}

// GetChats retrieves the account chats.
func (c *Client) GetChats(ctx context.Context) ([]Entity, error) {
	return c.GetEntities(ctx, FilterChat())
}

// GetChannels retrieves the account channels.
func (c *Client) GetChannels(ctx context.Context) ([]Entity, error) {
	return c.GetEntities(ctx, FilterChannel())
}

// GetEntities ensures that storage is populated, then iterates through storage
// peers calling filterFn for each peer. The filterFn should return Entity and
// true, if the peer satisfies the criteria, or nil and false, otherwise.
func (c *Client) GetEntities(ctx context.Context, filterFn FilterFunc) ([]Entity, error) {
	ctx, task := trace.NewTask(ctx, "GetEntities")
	defer task.End()

	if err := c.ensureDlgStoragePopulated(ctx); err != nil {
		return nil, err
	}

	peerIter, err := c.storage.Iterate(ctx)
	if err != nil {
		return nil, err
	}
	defer peerIter.Close()

	var ee []Entity

	for peerIter.Next(ctx) {
		ent, ok := filterFn(peerIter.Value())
		if !ok {
			continue
		}
		ee = append(ee, ent)
	}
	if err := peerIter.Err(); err != nil {
		return nil, err
	}
	return ee, nil
}

// ensureDlgStoragePopulated ensures that the dialog storage is populated
// within defCacheEvict duration.
func (c *Client) ensureDlgStoragePopulated(ctx context.Context) error {
	if cached, err := c.cache.Get(cacheDlgStorage); err == nil && cached.(bool) {
		trace.Log(ctx, "cache", "hit")
		return nil
	}
	// populating the storage
	trace.Log(ctx, "cache", "miss")

	dlgIter := dialogs.NewQueryBuilder(c.cl.API()).
		GetDialogs().
		BatchSize(defBatchSize).
		Iter()
	if err := storage.CollectPeers(c.storage).Dialogs(ctx, dlgIter); err != nil {
		return err
	}
	if err := c.cache.SetWithExpire(cacheDlgStorage, true, defCacheEvict); err != nil {
		return err
	}

	return nil
}

// CreateChat creates a Chat (not a Mega- or Gigagroup).
//
// Example
//
//	 if err := cl.CreateChat(ctx, "mtproto-test",123455678, 312849128); err != nil {
//			return err
//		}
func (c *Client) CreateChat(ctx context.Context, title string, userIDs ...int64) error {
	if len(userIDs) == 0 {
		return errors.New("at least one user is required")
	}

	var others = make([]tg.InputUserClass, len(userIDs))
	for i := range userIDs {
		others[i] = &tg.InputUser{UserID: userIDs[i]}
	}

	var users = append([]tg.InputUserClass{&tg.InputUserSelf{}}, others...)

	var resp tg.Updates
	if err := c.cl.Invoke(ctx,
		&tg.MessagesCreateChatRequest{
			Users: users,
			Title: title,
		},
		&resp,
	); err != nil {
		return err
	}
	return nil
}

// FindChat returns a chat with ID.
func (c *Client) FindChat(ctx context.Context, id int64) (*tg.Chat, error) {
	chat, err := c.GetEntities(ctx, FilterAnd(FilterChat(), FilterPeer(id)))
	if err != nil {
		return nil, err
	}
	if len(chat) == 0 {
		return nil, storage.ErrPeerNotFound
	}
	return chat[0].(*tg.Chat), nil
}

// FindChannel returns a channel with ID.
func (c *Client) FindChannel(ctx context.Context, id int64) (*tg.Channel, error) {
	chans, err := c.GetEntities(ctx, FilterAnd(FilterChannel(), FilterPeer(id)))
	if err != nil {
		return nil, err
	}
	if len(chans) == 0 {
		return nil, storage.ErrPeerNotFound
	}

	return chans[0].(*tg.Channel), nil
}
