package cache

import (
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type TieredCache[V any, K comparable] struct {
	hot         atomic.Value // just a *sync.Map (for poiter swap)
	counters    atomic.Value // just a *sync.Map
	cold        *ShardedCache[V, K]
	threshold   int64
	promoChan   chan promoTask[V, K]
	marshalFunc func(V) ([]byte, error)
}

type HotEntry[V any] struct {
	Data      V
	JSONBytes []byte
}

type promoTask[V any, K comparable] struct {
	key K
	val V
}

func NewTieredCache[V any, K comparable](lruSize int, lruShardCount int, promoteThreshold int64, promoChanBuffer int, marshal func(V) ([]byte, error), hashFunc func(K) uint32) *TieredCache[V, K] {
	tc := &TieredCache[V, K]{
		cold:        NewShardedCache[V](lruSize, lruShardCount, hashFunc),
		threshold:   promoteThreshold,
		promoChan:   make(chan promoTask[V, K], promoChanBuffer),
		marshalFunc: marshal,
	}
	tc.hot.Store(&sync.Map{})
	tc.counters.Store(&sync.Map{})

	go tc.janitor()
	go tc.promotionWorker()
	return tc
}
func (tc *TieredCache[V, K]) janitor() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tc.hot.Store(&sync.Map{})
		tc.counters.Store(&sync.Map{})
	}
}

func (tc *TieredCache[V, K]) promotionWorker() {
	for task := range tc.promoChan {
		tc.promote(task.key, task.val)
	}
}
func (tc *TieredCache[V, K]) promote(key K, val V) {
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

func (tc *TieredCache[V, K]) Get(key K) (V, []byte, bool) {
	hot := tc.hot.Load().(*sync.Map)

	if val, ok := hot.Load(key); ok {
		entry := val.(*HotEntry[V])
		return entry.Data, entry.JSONBytes, true
	}
	val, ok := tc.cold.Get(key)
	if ok {
		select {
		case tc.promoChan <- promoTask[V, K]{key, val}:
		default:
		}
	}
	return val, nil, ok
}

func (tc *TieredCache[V, K]) Add(key K, val V) {
	tc.cold.Add(key, val)
}

type ShardedCache[V any, K comparable] struct {
	shards   []*lru.Cache[K, V]
	mask     uint32
	hashFunc func(K) uint32
}

func NewShardedCache[V any, K comparable](totalSize int, shardCount int, hash func(K) uint32) *ShardedCache[V, K] {
	sc := &ShardedCache[V, K]{
		shards:   make([]*lru.Cache[K, V], shardCount),
		mask:     uint32(shardCount - 1),
		hashFunc: hash,
	}
	for i := range shardCount {
		sc.shards[i], _ = lru.New[K, V](totalSize / shardCount)
	}
	return sc
}

func (sc *ShardedCache[V, K]) getShard(key K) *lru.Cache[K, V] {
	return sc.shards[sc.hashFunc(key)&sc.mask]
}

func (sc *ShardedCache[V, K]) Get(key K) (V, bool) {
	return sc.getShard(key).Get(key)
}

func (sc *ShardedCache[V, K]) Add(key K, val V) {
	sc.getShard(key).Add(key, val)
}
