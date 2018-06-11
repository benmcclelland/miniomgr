package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coreos/bbolt"
)

var (
	keysBucket = "keys"
	dbpath     = "/var/miniomgr"
	dbfile     = "mm.db"
)

type boltdb struct {
	db *bolt.DB
}

func NewBoltDB() (*boltdb, error) {
	b := &boltdb{}
	err := b.init()
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (b *boltdb) init() error {
	err := os.MkdirAll(dbpath, 0700)
	if err != nil {
		return err
	}
	b.db, err = bolt.Open(filepath.Join(dbpath, dbfile), 0600,
		&bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(keysBucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
}

type userkeys struct {
	AccessKey string
	SecretKey string
}

func (b *boltdb) GetKeys(user string) (string, string, error) {
	var keybytes []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(keysBucket))
		keybytes = bkt.Get([]byte(user))
		return nil
	})
	if err != nil {
		return "", "", err
	}

	var keys userkeys
	err = json.Unmarshal(keybytes, &keys)
	if err != nil {
		return "", "", fmt.Errorf("decoding user %q: %v", user, err)
	}

	return keys.AccessKey, keys.SecretKey, nil
}

func (b *boltdb) SetKeys(user, access, secret string) error {
	keys := userkeys{
		AccessKey: access,
		SecretKey: secret,
	}

	encoded, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(keysBucket))
		err := bkt.Put([]byte(user), encoded)
		return err
	})
}

func (b *boltdb) Close() {
	b.Close()
}
