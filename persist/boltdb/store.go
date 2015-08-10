package boltdb

import (
	"errors"
	"time"

	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/boltdb/bolt"
	"github.com/tag1consulting/pipeviz/persist/item"
)

const (
	fileMode = 0600
)

var (
	// The name of the bucket to use.
	bucketName = []byte("journal")
)

// Represents a single BoltDB file storage backend for the append-only log.
// Bolt, a pure Go implementation inspired by LMDB, is a k/v store and thus
// provides more than we actually need, but it's an easy starting point.
type BoltStore struct {
	conn *bolt.DB
	path string
}

// NewBoltStore creates a handle to a BoltDB-backed log store
func NewBoltStore(path string) (*BoltStore, error) {
	// Allow 1s timeout on obtaining a file lock
	b, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}

	store := &BoltStore{
		conn: b,
		path: path,
	}

	// initialize the one bucket we use
	if err := store.init(); err != nil {
		store.conn.Close()
		return nil, err
	}

	return store, nil
}

// init sets up the journal bucket in the boltdb backend.
func (b *BoltStore) init() error {
	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}

	// Create all the buckets
	if _, err := tx.CreateBucketIfNotExists(bucketName); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Get returns the item associated with the given index.
func (b *BoltStore) Get(idx uint64) (*item.Log, error) {
	tx, err := b.conn.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(bucketName)
	val := bucket.Get(uint64ToBytes(idx))

	if val == nil {
		return nil, errors.New("index not found")
	}

	l := &item.Log{}
	if err := decodeMsgPack(val, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Append pushes a log item into the boltdb storage.
func (b *BoltStore) Append(log *item.Log) error {
	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// no need to sync b/c the conn.Begin(true) call will block
	bucket := tx.Bucket(bucketName)

	log.Index, err = bucket.NextSequence()
	if err != nil {
		return err
	}

	key := uint64ToBytes(log.Index)
	val, err := encodeMsgPack(log)
	if err != nil {
		return err
	}

	if err := bucket.Put(key, val.Bytes()); err != nil {
		return err
	}

	return tx.Commit()
}

// Count reports the number of items in the journal by opening a db cursor to
// grab the last item from the bucket. Because we're append-only, this is
// guaranteed to be the last one, and thus its index is the count..
func (b *BoltStore) Count() (uint64, error) {
	tx, err := b.conn.Begin(false)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	curs := tx.Bucket(bucketName).Cursor()
	if last, _ := curs.Last(); last == nil {
		return 0, nil
	} else {
		return bytesToUint64(last), nil
	}
}
