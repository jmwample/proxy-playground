package proxy

import (
	"context"
	"sync"
	"time"
)

type mContext struct {
	mu      sync.Mutex
	mainCtx context.Context
	ctx     context.Context
	done    chan struct{}
	err     error
}

func mergeContexts(mainCtx, ctx context.Context) context.Context {
	c := &mContext{mainCtx: mainCtx, ctx: ctx, done: make(chan struct{})}
	go c.run()
	return c
}

func (c *mContext) Done() <-chan struct{} {
	return c.done
}

func (c *mContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *mContext) Deadline() (deadline time.Time, ok bool) {
	var d time.Time
	d1, ok1 := c.ctx.Deadline()
	d2, ok2 := c.mainCtx.Deadline()
	if ok1 && d1.UnixNano() < d2.UnixNano() {
		d = d1
	} else if ok2 {
		d = d2
	}
	return d, ok1 || ok2
}

func (c *mContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

func (c *mContext) run() {
	var doneCtx context.Context
	select {
	case <-c.mainCtx.Done():
		doneCtx = c.mainCtx
	case <-c.ctx.Done():
		doneCtx = c.ctx
	case <-c.done:
		return
	}

	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = doneCtx.Err()
	c.mu.Unlock()
	close(c.done)
}
