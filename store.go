package main

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var threadsBucket = []byte("threads")

type storedSession struct {
	SessionID  string    `json:"session_id"`
	WorkingDir string    `json:"working_dir"`
	CreatedAt  time.Time `json:"created_at"`
}

type sessionStore struct {
	db *bolt.DB
}

func OpenStore(path string) (*sessionStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(threadsBucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init session store: %w", err)
	}

	return &sessionStore{db: db}, nil
}

func (s *sessionStore) Close() error {
	return s.db.Close()
}

func sessionBucketName(botName string) []byte {
	return []byte("sessions:" + botName)
}

func (s *sessionStore) PutSession(botName, channelID string, entry sessionEntry) error {
	data, err := json.Marshal(storedSession{
		SessionID:  entry.sessionID,
		WorkingDir: entry.workingDir,
		CreatedAt:  entry.createdAt,
	})
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(sessionBucketName(botName))
		if err != nil {
			return err
		}
		return b.Put([]byte(channelID), data)
	})
}

func (s *sessionStore) DeleteSession(botName, channelID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionBucketName(botName))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(channelID))
	})
}

func (s *sessionStore) AllSessions(botName string) map[string]sessionEntry {
	result := make(map[string]sessionEntry)

	_ = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionBucketName(botName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var stored storedSession
			if err := json.Unmarshal(v, &stored); err != nil {
				return nil // skip corrupt entries
			}
			result[string(k)] = sessionEntry{
				sessionID:  stored.SessionID,
				workingDir: stored.WorkingDir,
				createdAt:  stored.CreatedAt,
			}
			return nil
		})
	})

	return result
}

func (s *sessionStore) PurgeExpiredSessions(botName string, ttl time.Duration) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionBucketName(botName))
		if b == nil {
			return nil
		}

		var expired [][]byte
		_ = b.ForEach(func(k, v []byte) error {
			var stored storedSession
			if err := json.Unmarshal(v, &stored); err != nil {
				expired = append(expired, append([]byte(nil), k...))
				return nil
			}
			if time.Since(stored.CreatedAt) > ttl {
				expired = append(expired, append([]byte(nil), k...))
			}
			return nil
		})

		for _, k := range expired {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *sessionStore) PutThread(channelID, botName string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(threadsBucket)
		return b.Put([]byte(channelID), []byte(botName))
	})
}

func (s *sessionStore) AllThreads() map[string]string {
	result := make(map[string]string)

	_ = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(threadsBucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			result[string(k)] = string(v)
			return nil
		})
	})

	return result
}
