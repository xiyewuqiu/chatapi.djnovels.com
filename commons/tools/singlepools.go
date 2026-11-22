package tools

import (
	"sync"
	"sync/atomic"
)

type SinglePools struct {
	pools     []*SinglePool
	lock      *sync.RWMutex
	isBlocked bool
}
type SinglePool struct {
	taskChan  chan func()
	isActived atomic.Bool
	isBlocked atomic.Bool
}

func NewSinglePools(count int, isBlocked bool) *SinglePools {
	return &SinglePools{
		pools:     make([]*SinglePool, count),
		lock:      &sync.RWMutex{},
		isBlocked: isBlocked,
	}
}

// GetPool 按 key 取（必要时创建）一个单例池。
//
// 这里是双重检查锁，fast path **必须持读锁**：pools.pools 是普通切片，
// 无锁读一个正在被其他 goroutine 写入的元素是数据竞争
// （go test -race + 并发压测能稳定复现 GetPool:30 与 GetPool:41 的竞争）。
//
// 风险不只是"竞争本身"：可能读到非 nil 但 taskChan 还没初始化的池，
// 那时 Submit 会往 nil channel 发送并永久阻塞——比 panic 更难查。
func (pools *SinglePools) GetPool(key string) *SinglePool {
	hash := HashStr(key)
	index := int(hash % uint32(len(pools.pools)))

	pools.lock.RLock()
	pool := pools.pools[index]
	pools.lock.RUnlock()
	if pool != nil {
		return pool
	}

	pools.lock.Lock()
	defer pools.lock.Unlock()
	// 二次检查：拿到写锁之前可能已经有人建好了。
	if pool = pools.pools[index]; pool == nil {
		pool = &SinglePool{
			taskChan: make(chan func(), 100),
		}
		pool.isActived.Store(true)
		pool.isBlocked.Store(pools.isBlocked)
		pools.pools[index] = pool
		pool.Start()
	}
	return pool
}

func (pool *SinglePool) Start() {
	go func() {
		for pool.isActived.Load() {
			task := <-pool.taskChan
			func() {
				defer func() {
					recover()
				}()
				task()
			}()
		}
		close(pool.taskChan)
	}()
}

func (pool *SinglePool) Submit(task func()) bool {
	if pool.isActived.Load() {
		if pool.isBlocked.Load() {
			pool.taskChan <- task
			return true
		} else {
			select {
			case pool.taskChan <- task:
				return true
			default:
				return false
			}
		}
	}
	return false
}
