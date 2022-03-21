package refstore

// A RetentionPolicy describes how various reference store implementations
// should handle deletion of old references.
//
// Note, that the retention policy only keeps track of Identifiers, not
// of the physical storage location. Reference store implementation need
// to properly act on the return values of Prime and Add, and remove the
// referenced files.
type RetentionPolicy interface {
	// Prime should be used to fill the internal access list on startup.
	// It returns the list of evicted file references (which the reference
	// store should then delete).
	//
	// Semantically, Prime is equivalent to calling Add multiple times:
	//
	//	// add multiple references at once
	//	Prime([]*FileRef{a, b, c})
	//	// add references individually
	//	Add(a); Add(b); Add(c)
	//
	// (Note: this example omits handling of evicted references.)
	Prime(refs []*FileRef) (evicted []*FileRef)

	// Touch marks an identifier as recently used.
	Touch(id Identifier)

	// Add will insert a file reference into an internal store and return
	// file references which should be evicted. Depending on the chosen
	// implementation, the file argument can be part of the returned pruning
	// set.
	Add(ref *FileRef) (evicted []*FileRef)

	// Peek performa a lookup of the given identifier and returns a copy
	// returns a copy of the reference file, should it exist. Otherwise
	// this simply returns nil. In contract to Add and Touch, Peek should
	// not have any side effects.
	//
	// Note: Peek is intended to help in testing. Calling Peek is safe to
	// use concurrently, but it makes no guarantees as to whether the
	// returned value still exists in the access list after the next call
	// to Add().
	Peek(id Identifier) (existing *FileRef)
}

// Known retention policies.
var (
	_ RetentionPolicy = (*KeepForever)(nil)
	_ RetentionPolicy = (*PurgeOnStart)(nil)
	_ RetentionPolicy = (*AccessList)(nil)
)

// KeepForever is a no-op retention policy, i.e. reference stores
// should keeps all files forever.
type KeepForever struct{}

func (*KeepForever) Prime([]*FileRef) []*FileRef { return nil }
func (*KeepForever) Touch(Identifier)            {}
func (*KeepForever) Add(f *FileRef) []*FileRef   { return nil }
func (*KeepForever) Peek(Identifier) *FileRef    { return nil }

// PurgeOnStart will delete all files when the reference store is
// instantiated. This typically happens once on service startup.
type PurgeOnStart struct{}

func (*PurgeOnStart) Prime(f []*FileRef) []*FileRef { return f }
func (*PurgeOnStart) Touch(Identifier)              {}
func (*PurgeOnStart) Add(f *FileRef) []*FileRef     { return nil }
func (*PurgeOnStart) Peek(Identifier) *FileRef      { return nil }
