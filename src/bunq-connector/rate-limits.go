package bunqconnector

import (
	"context"
	"time"
)

type rateLimits struct {
	getRequests           <-chan interface{}
	postRequests          <-chan interface{}
	putRequests           <-chan interface{}
	sessionServerRequests <-chan interface{}
}

type rateLimiter interface {
	forGet() <-chan interface{}
	forPost() <-chan interface{}
	forPut() <-chan interface{}
	forSessionServer() <-chan interface{}
}

type defaultRateLimiter struct {
	channelForGet           <-chan interface{}
	channelForPost          <-chan interface{}
	channelForPut           <-chan interface{}
	channelForSessionServer <-chan interface{}
}

func newDefaultRateLimiter(ctx context.Context) rateLimiter {
	res := new(defaultRateLimiter)
	res.channelForGet = buildLimitChannel(ctx, 3, 3*time.Second)
	res.channelForPost = buildLimitChannel(ctx, 5, 3*time.Second)
	res.channelForPut = buildLimitChannel(ctx, 3, 3*time.Second)
	res.channelForSessionServer = buildLimitChannel(ctx, 1, 30*time.Second)
	return *res
}

func buildLimitChannel(ctx context.Context, amount int, per time.Duration) <-chan interface{} {
	burstyLimiter := make(chan interface{}, amount)
	for i := 0; i < amount; i++ {
		burstyLimiter <- time.Now()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case x := <-time.Tick(per):
				burstyLimiter <- x
			}
		}
	}()

	return burstyLimiter
}

func (limiter defaultRateLimiter) forGet() <-chan interface{} {
	return limiter.channelForGet
}

func (limiter defaultRateLimiter) forPost() <-chan interface{} {
	return limiter.channelForPost
}

func (limiter defaultRateLimiter) forPut() <-chan interface{} {
	return limiter.channelForPut
}

func (limiter defaultRateLimiter) forSessionServer() <-chan interface{} {
	return limiter.channelForSessionServer
}
