package collector

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 게이트가 실제로 동시 실행을 제한하는지 확인한다.
// Engine 없이 슬롯 획득/해제만 검증하기 위해 내부 채널을 직접 쓴다.
func (g *analysisGate) runForTest(fn func()) {
	g.slots <- struct{}{}
	defer func() { <-g.slots }()
	fn()
}

func TestAnalysisGateSerializesWhenLimitIsOne(t *testing.T) {
	g := newAnalysisGate(1)

	var inFlight, maxSeen int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.runForTest(func() {
				cur := atomic.AddInt32(&inFlight, 1)
				for {
					old := atomic.LoadInt32(&maxSeen)
					if cur <= old || atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				atomic.AddInt32(&inFlight, -1)
			})
		}()
	}
	wg.Wait()

	if maxSeen != 1 {
		t.Errorf("동시 실행 최대 %d개, want 1 (직렬화되지 않았다)", maxSeen)
	}
}

func TestAnalysisGateRespectsLimit(t *testing.T) {
	const limit = 3
	g := newAnalysisGate(limit)

	var inFlight, maxSeen int32
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.runForTest(func() {
				cur := atomic.AddInt32(&inFlight, 1)
				for {
					old := atomic.LoadInt32(&maxSeen)
					if cur <= old || atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				atomic.AddInt32(&inFlight, -1)
			})
		}()
	}
	wg.Wait()

	if maxSeen > limit {
		t.Errorf("동시 실행 최대 %d개, 제한 %d 초과", maxSeen, limit)
	}
}

// 0이나 음수는 1로 보정해야 한다(설정 미지정 시 직렬 실행).
func TestAnalysisGateDefaultsToOne(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if got := cap(newAnalysisGate(n).slots); got != 1 {
			t.Errorf("newAnalysisGate(%d) 슬롯=%d, want 1", n, got)
		}
	}
	if got := cap(newAnalysisGate(4).slots); got != 4 {
		t.Errorf("newAnalysisGate(4) 슬롯=%d, want 4", got)
	}
}

// 대기 중인 요청이 거부되지 않고 모두 수행되어야 한다(처리 누락 방지).
func TestAnalysisGateDoesNotDropWork(t *testing.T) {
	g := newAnalysisGate(2)
	var done int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.runForTest(func() { atomic.AddInt32(&done, 1) })
		}()
	}
	wg.Wait()
	if done != 50 {
		t.Errorf("수행된 작업 %d개, want 50 (누락됐다)", done)
	}
}
