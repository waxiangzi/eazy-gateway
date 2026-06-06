package db

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// TrafficStats holds persisted byte counters for a tunnel.
type TrafficStats struct {
	TotalBytesIn  uint64 `json:"totalBytesIn"`
	TotalBytesOut uint64 `json:"totalBytesOut"`
}

// TrafficSample holds a single point-in-time snapshot of a tunnel's cumulative traffic.
type TrafficSample struct {
	Timestamp int64  `json:"timestamp"`
	BytesIn   uint64 `json:"bytesIn"`
	BytesOut  uint64 `json:"bytesOut"`
}

// GetTraffic retrieves persisted traffic stats for a tunnel.
// Returns nil if no stats have been stored yet.
func (db *DB) GetTraffic(tunnelID string) (*TrafficStats, error) {
	var stats *TrafficStats
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTraffic)
		data := b.Get([]byte(tunnelID))
		if data == nil {
			return nil
		}
		var s TrafficStats
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unmarshal traffic %q: %w", tunnelID, err)
		}
		stats = &s
		return nil
	})
	return stats, err
}

// UpdateTraffic persists traffic stats for a tunnel.
func (db *DB) UpdateTraffic(tunnelID string, stats *TrafficStats) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTraffic)
		data, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("marshal traffic %q: %w", tunnelID, err)
		}
		return b.Put([]byte(tunnelID), data)
	})
}

// DeleteTraffic removes persisted traffic stats for a tunnel.
func (db *DB) DeleteTraffic(tunnelID string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketTraffic).Delete([]byte(tunnelID))
	})
}

// RecordTrafficSample stores a traffic snapshot for a tunnel.
func (db *DB) RecordTrafficSample(tunnelID string, sample *TrafficSample) error {
	key := fmt.Sprintf("%s/%d", tunnelID, sample.Timestamp)
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTrafficSamples)
		data, err := json.Marshal(sample)
		if err != nil {
			return fmt.Errorf("marshal traffic sample %q: %w", tunnelID, err)
		}
		return b.Put([]byte(key), data)
	})
}

// ListTrafficSamples returns all traffic samples for a tunnel since the given time.
func (db *DB) ListTrafficSamples(tunnelID string, since time.Time) ([]TrafficSample, error) {
	prefix := tunnelID + "/"
	cutoff := since.Unix()
	var samples []TrafficSample
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTrafficSamples)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if !strings.HasPrefix(string(k), prefix) {
				return nil
			}
			var s TrafficSample
			if err := json.Unmarshal(v, &s); err != nil {
				return nil
			}
			if s.Timestamp >= cutoff {
				samples = append(samples, s)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Timestamp < samples[j].Timestamp
	})
	return samples, nil
}

// CleanupTrafficSamples removes all traffic samples older than the given cutoff.
func (db *DB) CleanupTrafficSamples(olderThan time.Time) error {
	cutoff := olderThan.Unix()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTrafficSamples)
		if b == nil {
			return nil
		}
		var toDelete [][]byte
		if err := b.ForEach(func(k, v []byte) error {
			parts := strings.Split(string(k), "/")
			if len(parts) != 2 {
				return nil
			}
			ts, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil
			}
			if ts < cutoff {
				toDelete = append(toDelete, []byte(string(k)))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
