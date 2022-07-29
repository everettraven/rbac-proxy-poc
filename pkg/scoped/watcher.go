package scoped

import (
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

type ScopedWatcher struct {
	mutex   sync.Mutex
	result  chan watch.Event
	stopCh  chan struct{}
	stopped bool
}

func NewScopedWatcher(watches ...<-chan watch.Event) *ScopedWatcher {
	var wg sync.WaitGroup
	watchChannel := make(chan watch.Event)
	merge := func(c <-chan watch.Event) {
		for event := range c {
			watchChannel <- event
		}
		wg.Done()
	}

	wg.Add(len(watches))
	for _, watch := range watches {
		go merge(watch)
	}

	go func() {
		wg.Wait()
		close(watchChannel)
	}()

	return &ScopedWatcher{
		result:  watchChannel,
		stopCh:  make(chan struct{}),
		stopped: false,
	}
}

func (sw *ScopedWatcher) Stop() {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	if !sw.stopped {
		sw.stopped = true
		close(sw.stopCh)
	}
}

func (sw *ScopedWatcher) ResultChan() <-chan watch.Event {
	return sw.result
}
