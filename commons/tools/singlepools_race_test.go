package tools

import (
	"sync"
	"testing"
)

// GetPool 是双重检查锁：fast path 必须持读锁，否则与慢路径的写入构成数据竞争。
// 用 -race 跑这个测试才有意义：
//   go test -race ./commons/tools/
func TestSinglePoolsGetPoolIsRaceFree(t *testing.T) {
	pools := NewSinglePools(16, false)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				// 故意让多个 goroutine 撞同一个 key，制造"同时创建"的场景。
				pool := pools.GetPool("same-key")
				if pool == nil {
					t.Error("GetPool 不应返回 nil")
					return
				}
				// 同一 key 必须拿到同一个池。
				if other := pools.GetPool("same-key"); other != pool {
					t.Error("同一 key 返回了不同的池")
					return
				}
				_ = pools.GetPool("key-" + string(rune('a'+offset)))
			}
		}(g)
	}
	wg.Wait()
}

// 同一个 key 在并发下只能建出一个池：否则任务会被投进不同的 channel，
// 同一会话的消息顺序就乱了。
func TestSinglePoolsCreatesOnePoolPerKey(t *testing.T) {
	pools := NewSinglePools(8, false)

	var wg sync.WaitGroup
	results := make([]*SinglePool, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = pools.GetPool("contended")
		}(i)
	}
	wg.Wait()

	first := results[0]
	for i, pool := range results {
		if pool != first {
			t.Fatalf("第 %d 个 goroutine 拿到不同的池——同会话消息会乱序", i)
		}
	}
}
