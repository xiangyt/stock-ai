package utils

import "sync"

// ConcurrentExec 通用并发执行器。
//   - tasks   : 任务列表，每个元素会被 fn 处理
//   - maxGor  : 最大并发数，<= 0 表示不限制
//   - fn      : 对每个索引 i 执行的函数，返回 error，nil 表示成功
//
// fn 接收任务在 tasks 中的索引，方便直接写入结果切片。
// 返回第一个遇到的 error（如果所有任务都成功则返回 nil）。
func ConcurrentExec[T any](tasks []T, maxGor int, fn func(i int, t T) error) error {
	if len(tasks) == 0 {
		return nil
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	// 信号量限制并发
	sem := make(chan struct{}, maxGor)
	if maxGor <= 0 {
		sem = nil // 不限制
	}

	for i := range tasks {
		wg.Add(1)
		go func(idx int, t T) {
			defer wg.Done()

			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			if err := fn(idx, t); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(i, tasks[i])
	}

	wg.Wait()
	return firstErr
}
