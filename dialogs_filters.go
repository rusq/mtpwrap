package mtpwrap

import "github.com/gotd/contrib/storage"

type FilterFunc func(storage.Peer) (ent Entity, ok bool)

func FilterChat() FilterFunc {
	return func(peer storage.Peer) (Entity, bool) {
		if peer.Chat != nil {
			return peer.Chat, true
		} else if peer.Channel != nil && !peer.Channel.Broadcast {
			return peer.Channel, true
		}
		return nil, false
	}
}

func FilterChannel() FilterFunc {
	return func(peer storage.Peer) (Entity, bool) {
		if peer.Channel != nil && peer.Channel.Broadcast {
			return peer.Channel, true
		}
		return nil, false
	}
}

func FilterPeer(id int64) FilterFunc {
	return func(p storage.Peer) (ent Entity, ok bool) {
		if p.Channel != nil && p.Channel.ID == id {
			return p.Channel, true
		}
		if p.Chat != nil && p.Chat.ID == id {
			return p.Chat, true
		}
		return nil, false
	}
}

func FilterAnd(f1 FilterFunc, f2 FilterFunc) FilterFunc {
	return func(p storage.Peer) (ent Entity, ok bool) {
		r1, ok1 := f1(p)
		_, ok2 := f2(p)
		if ok1 && ok2 {
			return r1, ok1
		}
		return nil, false
	}
}
