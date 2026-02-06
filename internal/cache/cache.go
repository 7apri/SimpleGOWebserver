package cache

import (
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type TieredCache[K comparable, V any] struct {
	hot         atomic.Value // just a *sync.Map (for poiter swap)
	counters    atomic.Value // just a *sync.Map
	cold        *ShardedCache[K, V]
	threshold   int64
	promoChan   chan promoTask[K, V]
	marshalFunc func(V) ([]byte, error)
}

type HotEntry[V any] struct {
	Data      V
	JSONBytes []byte
}

type promoTask[K comparable, V any] struct {
	key K
	val V
}

func NewTieredCache[V any, K comparable](lruSize int, lruShardCount int, promoteThreshold int64, promoChanBuffer int, marshal func(V) ([]byte, error), hashFunc func(K) uint32) *TieredCache[K, V] {
	tc := &TieredCache[K, V]{
		cold:        NewShardedCache[K, V](lruSize, lruShardCount, hashFunc),
		threshold:   promoteThreshold,
		promoChan:   make(chan promoTask[K, V], promoChanBuffer),
		marshalFunc: marshal,
	}
	tc.hot.Store(&sync.Map{})
	tc.counters.Store(&sync.Map{})

	go tc.janitor()
	go tc.promotionWorker()
	return tc
}
func (tc *TieredCache[K, V]) janitor() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tc.hot.Store(&sync.Map{})
		tc.counters.Store(&sync.Map{})
	}
}

func (tc *TieredCache[K, V]) promotionWorker() {
	for task := range tc.promoChan {
		tc.promote(task.key, task.val)
	}
}
func (tc *TieredCache[K, V]) promote(key K, val V) {
	counters := tc.counters.Load().(*sync.Map)
	hot := tc.hot.Load().(*sync.Map)

	v, _ := counters.LoadOrStore(key, &atomic.Int64{})
	counter := v.(*atomic.Int64)

	hits := counter.Add(1)
	if hits == tc.threshold {
		b, err := tc.marshalFunc(val)
		if err != nil {
			return
		}
		hot.Store(key, &HotEntry[V]{
			Data:      val,
			JSONBytes: b,
		})
		counters.Delete(key)
	}
}

func (tc *TieredCache[K, V]) Get(key K) (V, []byte, bool) {
	hot := tc.hot.Load().(*sync.Map)

	if val, ok := hot.Load(key); ok {
		entry := val.(*HotEntry[V])
		return entry.Data, entry.JSONBytes, true
	}
	val, ok := tc.cold.Get(key)
	if ok {
		select {
		case tc.promoChan <- promoTask[K, V]{key, val}:
		default:
		}
	}
	return val, nil, ok
}

func (tc *TieredCache[K, V]) Add(key K, val V) {
	tc.cold.Add(key, val)
}

type ShardedCache[K comparable, V any] struct {
	shards   []*lru.Cache[K, V]
	mask     uint32
	hashFunc func(K) uint32
}

func NewShardedCache[K comparable, V any](totalSize int, shardCount int, hash func(K) uint32) *ShardedCache[K, V] {
	sc := &ShardedCache[K, V]{
		shards:   make([]*lru.Cache[K, V], shardCount),
		mask:     uint32(shardCount - 1),
		hashFunc: hash,
	}
	for i := range shardCount {
		sc.shards[i], _ = lru.New[K, V](totalSize / shardCount)
	}
	return sc
}

func (sc *ShardedCache[K, V]) getShard(key K) *lru.Cache[K, V] {
	return sc.shards[sc.hashFunc(key)&sc.mask]
}

func (sc *ShardedCache[K, V]) Get(key K) (V, bool) {
	return sc.getShard(key).Get(key)
}

func (sc *ShardedCache[K, V]) Add(key K, val V) {
	sc.getShard(key).Add(key, val)
}
