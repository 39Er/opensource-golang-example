package main

import (
	"fmt"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/pkg/errors"
	"log"
	"net/http"
	"time"
)

type stateChangeListener struct {
}

func (s *stateChangeListener) OnTransformToClosed(prev circuitbreaker.State, rule circuitbreaker.Rule) {
	fmt.Printf("rule.steategy: %+v, From %s to Closed, time: %v\n", rule.Strategy, prev.String(), time.Now())
}

func (s *stateChangeListener) OnTransformToOpen(prev circuitbreaker.State, rule circuitbreaker.Rule, snapshot interface{}) {
	fmt.Printf("rule.steategy: %+v, From %s to Open, snapshot: %.2f, time: %v\n", rule.Strategy, prev.String(), snapshot, time.Now())
}

func (s *stateChangeListener) OnTransformToHalfOpen(prev circuitbreaker.State, rule circuitbreaker.Rule) {
	fmt.Printf("rule.steategy: %+v, From %s to Half-Open, time: %v\n", rule.Strategy, prev.String(), time.Now())
}

func main() {
	err := sentinel.InitDefault()
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan struct{})
	fmt.Println(time.Now())
	circuitbreaker.RegisterStateChangeListeners(&stateChangeListener{})

	_, err = circuitbreaker.LoadRules([]*circuitbreaker.Rule{
		// Statistic time span=10s, recoveryTimeout=3s, slowRtUpperBound=50ms, maxSlowRequestRatio=50%
		{
			Resource:         "abc",
			Strategy:         circuitbreaker.SlowRequestRatio,
			StatIntervalMs:   10000,
			RetryTimeoutMs:   3000,
			MinRequestAmount: 10,
			MaxAllowedRtMs:   50,
			Threshold:        0.5,
		},
		// Statistic time span=10s, recoveryTimeout=3s, maxErrorRatio=50%
		{
			Resource:         "abc",
			Strategy:         circuitbreaker.ErrorRatio,
			StatIntervalMs:   10000,
			RetryTimeoutMs:   3000,
			MinRequestAmount: 10,
			Threshold:        0.5,
		},
		{
			Resource:         "baidu",
			Strategy:         circuitbreaker.SlowRequestRatio,
			StatIntervalMs:   10000,
			RetryTimeoutMs:   1000,
			MinRequestAmount: 10,
			Threshold:        0.5,
			MaxAllowedRtMs:   50,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Sentinel Go circuit breaking demo is running. You may see the pass/block metric in the metric log.")
	//官方样例
	/*go func() {
		for {
			e, b := sentinel.Entry("abc")
			if b != nil {
				//fmt.Println("g1blocked")
				time.Sleep(time.Duration(rand.Uint64()%20) * time.Millisecond)
			} else {
				if rand.Uint64()%20 > 10 {
					// Record current invocation as error.
					sentinel.TraceError(e, errors.New("biz error"))
				}
				//fmt.Println("g1passed")
				time.Sleep(time.Duration(rand.Uint64()%80+10) * time.Millisecond)
				e.Exit()
			}
		}
	}()*/
	/*go func() {
		for {
			e, b := sentinel.Entry("abc")
			if b != nil {
				//fmt.Println("g2blocked")
				time.Sleep(time.Duration(rand.Uint64()%20) * time.Millisecond)
			} else {
				//fmt.Println("g2passed")
				time.Sleep(time.Duration(rand.Uint64()%30) * time.Millisecond)
				e.Exit()
			}
		}
	}()*/

	go func() {
		for {
			result := circuitCall("baidu", func() error {
				now := time.Now()
				_, err := http.Get("https://www.baidu.com")
				fmt.Printf("cost:%s\n", time.Now().Sub(now))
				return err
			}, func(be *base.BlockError) error {
				fmt.Printf("block type:%s,message:%s,rule:%s\n", be.BlockType(), be.BlockMsg(), be.TriggeredRule().String())
				return nil
			})
			fmt.Printf("call result:%v\n", result)
		}
	}()

	<-ch

}

type CircuitResult struct {
	Err         error
	FallbackErr error
	IsBreak     bool
}

func circuitCall(resource string, fn func() error, fallback func(be *base.BlockError) error) CircuitResult {
	result := CircuitResult{}
	e, b := sentinel.Entry(resource)
	if b != nil {
		if fallback != nil {
			result.FallbackErr = fallback(b)
		}
		result.IsBreak = true
	} else {
		runErr := fn()
		if runErr != nil {
			sentinel.TraceError(e, errors.Wrap(runErr, fmt.Sprintf("failed to execute resource:%s:", resource)))
		}
		e.Exit()
		result.Err = runErr
	}
	return result
}
