package server

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/thomasmmitchell/doomsday"
)

type source struct {
	Core     *doomsday.Core
	Name     string
	Interval time.Duration
	nextRun  time.Time
}

//Bump sets nextRun to now + interval
func (s *source) Bump() {
	s.nextRun = time.Now().Add(s.Interval)
}

func (s *source) Refresh(mode string, logger *os.File) {
	fmt.Fprintf(logger, "Running %s populate of `%s'\n", mode, s.Name)
	startedAt := time.Now()
	err := s.Core.Populate()
	if err != nil {
		fmt.Fprintf(logger, "Error populating info from backend `%s': %s\n", s.Name, err)
	}
	fmt.Fprintf(logger, "Finished %s populate of `%s' after %s\n", mode, s.Name, time.Since(startedAt))
}

type sourceManager struct {
	//sources is a static view of the queue, such that the get calls don't need
	// to sync with the populate calls. Otherwise, we would need to read-lock on
	// get calls so that the order of the queue doesn't shift from underneath us
	// as the async scheduler is running
	sources []source
	queue   []*source
	lock    sync.RWMutex
	logger  *os.File
}

func newSourceManager(sources []source, logger *os.File) *sourceManager {
	if logger == nil {
		logger = os.Stderr
	}

	queue := make([]*source, 0, len(sources))
	for i := range sources {
		queue = append(queue, &sources[i])
	}

	return &sourceManager{
		sources: sources,
		queue:   queue,
		logger:  logger,
	}
}

func (s *sourceManager) BackgroundScheduler() {
	if len(s.sources) == 0 {
		return
	}

	go func() {
		for {
			//Because ad-hoc refreshes can happen asynchronously, this thread may wake
			//before its time to populate again because the refresh has pushed it back.
			//In that case, just reevaluate your sleep time and come back later
			s.lock.Lock()
			current := s.queue[0]
			shouldRun := !(time.Now().Before(current.nextRun))
			if shouldRun {
				current.Bump()
				s.sortQueue()
			}
			s.lock.Unlock()

			if shouldRun {
				current.Refresh("scheduled", s.logger)
			}

			s.lock.RLock()
			nextTime := s.queue[0].nextRun
			s.lock.RUnlock()
			time.Sleep(time.Until(nextTime))
		}
	}()
}

func (s *sourceManager) sortQueue() {
	//This is a naive sort when only one thing can change order. Also, we should
	//be using a linked-list heap for performance. But considering that the number
	//of backends will be in the single digits... I really don't care about either
	//of those things. I've instead decided to fill the lines of code that I've
	//saved with that decision by writing this comment.
	sort.Slice(s.queue,
		func(i, j int) bool {
			return s.queue[i].nextRun.Before(s.queue[j].nextRun)
		},
	)
}

func (s *sourceManager) Data() []doomsday.CacheItem {
	items := []doomsday.CacheItem{}
	for _, source := range s.sources {
		data := source.Core.Cache().Map()
		for k, v := range data {
			items = append(items, doomsday.CacheItem{
				BackendName: source.Name,
				Path:        k,
				CommonName:  v.Subject.CommonName,
				NotAfter:    v.NotAfter.Unix(),
			})
		}
	}

	return items
}

func (s *sourceManager) RefreshAll() {
	//How long must have passed before we'll refresh a backend by request to avoid spamming
	const tooRecentThreshold time.Duration = time.Minute
	for _, current := range s.sources {
		s.lock.Lock()
		lastRun := current.nextRun.Add(-current.Interval)
		cutoffTime := time.Now().Add(-(tooRecentThreshold))
		shouldRun := lastRun.Before(cutoffTime)
		if shouldRun {
			current.Bump()
			s.sortQueue()
		}
		s.lock.Unlock()

		if shouldRun {
			current.Refresh("ad-hoc", s.logger)
		}
	}
}