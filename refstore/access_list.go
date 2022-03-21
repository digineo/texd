package refstore

import (
	"errors"
	"sync"

	list "github.com/bahlo/generic-list-go"
)

// FileRef represents a reference file. It does not contain any file data,
// only enough meta data to find the file content in a reference store (by
// its ID), and make decisions about evictions (using its Size).
type FileRef struct {
	ID   Identifier
	Size int
}

type quota struct{ cur, max int }

func (q quota) Satisfied() bool {
	return q.max >= 0 && q.cur <= q.max
}

// ErrInvalidAccessListConfig is returned from NewAccessList when the
// quota arguments (maxItems and totalSizeQuota) are both <= 0.
var ErrInvalidAccessListConfig = errors.New("invalid access list configuration, max. item count and file size can't both be infinite")

// AccessList implements a size- and/or space-limited RetentionPolicy.
// Size-limited means, that the AccessList may just accept a limited number
// of entries, while the space limitation constrains the total file size
// of all entries.
//
// Reference file usage (through Touch and Add) moves entries to the front
// of the access list, while adding entries may evict entries from the back
// when limits are exceeded. In doing so, AccessList acts as cache with
// LRU semantics.
type AccessList struct {
	items *list.List[*FileRef]
	cache map[Identifier]*list.Element[*FileRef]
	mu    *sync.Mutex

	maxItems  quota
	totalSize quota
}

func NewAccessList(maxItems, totalSizeQuota int) (*AccessList, error) {
	if maxItems <= 0 && totalSizeQuota <= 0 {
		return nil, ErrInvalidAccessListConfig
	}

	return &AccessList{
		items:     list.New[*FileRef](),
		cache:     make(map[Identifier]*list.Element[*FileRef]),
		mu:        &sync.Mutex{},
		maxItems:  quota{0, maxItems},
		totalSize: quota{0, totalSizeQuota},
	}, nil
}

// Prime implements the RetentionPolicy interface.
func (a *AccessList) Prime(init []*FileRef) (evicted []*FileRef) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, f := range init {
		evicted = append(evicted, a.add(f)...)
	}
	return evicted
}

// Peek implements the RetentionPolicy interface.
func (a *AccessList) Peek(id Identifier) *FileRef {
	a.mu.Lock()
	defer a.mu.Unlock()

	if e, ok := a.cache[id]; ok {
		return &FileRef{
			ID:   e.Value.ID,
			Size: e.Value.Size,
		}
	}
	return nil
}

// Touch implements the RetentionPolicy interface.
func (a *AccessList) Touch(id Identifier) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if e := a.cache[id]; e != nil {
		a.items.MoveToFront(e)
	}
}

// Add implements the RetentionPolicy interface.
func (a *AccessList) Add(f *FileRef) (evicted []*FileRef) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.add(f)
}

// add inserts f without locking.
func (a *AccessList) add(f *FileRef) (evicted []*FileRef) {
	if e := a.cache[f.ID]; e != nil {
		a.items.MoveToFront(e)
		return nil
	}

	a.cache[f.ID] = a.items.PushFront(f)
	a.maxItems.cur++
	a.totalSize.cur += f.Size

	for !a.totalSize.Satisfied() {
		if ev := a.truncateItem(); ev == nil {
			break
		} else {
			evicted = append(evicted, ev)
		}
	}
	for !a.maxItems.Satisfied() {
		if ev := a.truncateItem(); ev == nil {
			break
		} else {
			evicted = append(evicted, ev)
		}
	}
	return evicted
}

// truncateItem removes and returns one item from the back, without locking,
// if there's at least one item in the access list (otherwise it does nothing).
func (a *AccessList) truncateItem() (evicted *FileRef) {
	back := a.items.Back()
	if back == nil || a.items.Len() == 1 {
		return
	}
	f := a.items.Remove(back)
	delete(a.cache, f.ID)
	a.maxItems.cur--
	a.totalSize.cur -= f.Size
	return f
}
