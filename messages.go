package mtpwrap

import (
	"context"
	"fmt"
	"log"
	"runtime/trace"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/query"
	qmsg "github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
)

// SearchAllMyMessages returns the current authorized user messages from chat or
// channel `dlg`.  For each API call, the callback function will be invoked, if
// not nil.
func (c *Client) SearchAllMyMessages(ctx context.Context, dlg Entity, cb func(n int)) ([]qmsg.Elem, error) {
	return c.SearchAllMessages(ctx, dlg, &tg.InputPeerSelf{}, cb)
}

// SearchAllMessages search messages in the chat or channel `dlg`. It finds ALL
// messages from the person `who`. returns a slice of message.Elem. For each API
// call, the callback function will be invoked, if not nil.
func (c *Client) SearchAllMessages(ctx context.Context, dlg Entity, who tg.InputPeerClass, cb func(n int)) ([]qmsg.Elem, error) {
	q := c.Query(dlg).FromID(who).Filter(&tg.InputMessagesFilterEmpty{})

	return c.QueryMessages(ctx, dlg, q, cb)
}

func (c *Client) Query(e Entity) *qmsg.SearchQueryBuilder {
	return query.Messages(c.cl.API()).Search(AsInputPeer(e)).BatchSize(defBatchSize)
}

func (c *Client) QueryMessages(ctx context.Context, dlg Entity, q Iterator, cb func(int)) ([]qmsg.Elem, error) {
	if cached, err := c.cache.Get(cacheKey(dlg.GetID())); err == nil {
		msgs := cached.([]qmsg.Elem)
		if cb != nil {
			cb(len(msgs))
		}
		return msgs, nil
	}

	elems, err := collectMessages(ctx, q, cb)
	if err != nil {
		return nil, err
	}

	if err := c.cache.Set(cacheKey(dlg.GetID()), elems); err != nil {
		return nil, err
	}
	return elems, err
}

func (c *Client) DeleteMessages(ctx context.Context, dlg Entity, messages []qmsg.Elem) (int, error) {
	ctx, task := trace.NewTask(ctx, "DeleteMessages")
	defer task.End()

	if DlgType(dlg) == DUnknown {
		return 0, fmt.Errorf("unsupported dialog type: %T", dlg)
	}

	ids := splitBy(defBatchSize, messages, func(i int) int { return messages[i].Msg.GetID() })
	trace.Logf(ctx, "delete", "split chunks: %d", len(ids))

	// clearing cache.
	if c.cache.Remove(cacheKey(dlg.GetID())) {
		trace.Log(ctx, "delete", "cache cleared")
	}

	deleter := message.NewSender(c.cl.API()).To(AsInputPeer(dlg)).Revoke()
	total := 0
	for _, chunk := range ids {
		resp, err := deleter.Messages(ctx, chunk...)
		if err != nil {
			trace.Logf(ctx, "api", "revoke error: %s", err)
			return 0, fmt.Errorf("failed to delete: %w", err)
		}
		total += resp.GetPtsCount()
	}
	trace.Log(ctx, "delete", "ok")
	return total, nil
}

func AsInputPeer(ent Entity) tg.InputPeerClass {
	switch e := ent.(type) {
	case *tg.Channel:
		return e.AsInputPeer()
	case *tg.Chat:
		return e.AsInputPeer()
	}
	log.Panicf("invalid type %T", ent)
	// unreachable, but keeps compiler happy
	return nil
}

// splitBy splits the chunk input of M items to X chunks of `n` items.
// For each element of input, the fn is called, that should return
// the value.
func splitBy[T, S any](n int, input []S, fn func(i int) T) [][]T {
	var out [][]T = make([][]T, 0, len(input)/n)
	var chunk []T
	for i := range input {
		if i > 0 && i%n == 0 {
			out = append(out, chunk)
			chunk = make([]T, 0, n)
		}
		chunk = append(chunk, fn(i))
	}
	if len(chunk) > 0 {
		out = append(out, chunk)
	}
	return out
}

type Iterator interface {
	Iter() *qmsg.Iterator
}

// collectMessages is the copy/pasta from the td/telegram/message package with added
// optional callback function. It creates iterator and collects all elements to
// slice, calling callback function for each iteration, if it's not nil.
func collectMessages(ctx context.Context, b Iterator, cb func(n int)) ([]qmsg.Elem, error) {
	iter := b.Iter()
	c, err := iter.Total(ctx)
	if err != nil {
		return nil, fmt.Errorf("get total: %w", err)
	}

	r := make([]qmsg.Elem, 0, c)
	if err := ForEachMessage(ctx, b, func(m qmsg.Elem) error {
		r = append(r, m)
		if cb != nil {
			cb(1)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("collect messages: %w", err)
	}
	return r, nil
}

func ForEachMessage(ctx context.Context, b Iterator, cb func(m qmsg.Elem) error) error {
	iter := b.Iter()
	for iter.Next(ctx) {
		if err := cb(iter.Value()); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}
	return nil
}
