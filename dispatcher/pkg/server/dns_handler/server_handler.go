//     Copyright (C) 2020-2021, IrineSistiana
//
//     This file is part of mosdns.
//
//     mosdns is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     mosdns is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package dns_handler

import (
	"context"
	"github.com/IrineSistiana/mosdns/v2/dispatcher/handler"
	"github.com/IrineSistiana/mosdns/v2/dispatcher/pkg/concurrent_limiter"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"sync"
	"testing"
)

type Handler interface {
	// ServeDNS uses ctx to control deadline, exchanges qCtx, and writes response to w.
	ServeDNS(ctx context.Context, qCtx *handler.Context, w ResponseWriter)
}

// ResponseWriter can write msg to the client.
type ResponseWriter interface {
	Write(m *dns.Msg) (n int, err error)
}

type DefaultHandler struct {
	// Logger is used for logging. A nil value will disable logging.
	Logger *zap.Logger

	// Entry is the entry ExecutablePlugin's tag. This cannot be nil.
	Entry handler.ExecutableChainNode

	// ConcurrentLimit controls the max concurrent queries for the DefaultHandler.
	// If ConcurrentLimit <= 0, means no limit.
	// When calling DefaultHandler.ServeDNS(), if a query exceeds the limit, it will wait on a FIFO queue until
	// - its ctx is done or currently there are more than 3 x ConcurrentLimit queries waiting -> The query will be dropped silently.
	// - it can be proceeded -> Normal procedure.
	ConcurrentLimit int

	// RecursionAvailable sets the dns.Msg.RecursionAvailable flag globally.
	RecursionAvailable bool

	initOnce sync.Once // init the followings
	logger   *zap.Logger
	limiter  *concurrent_limiter.ConcurrentLimiter // if it's nil, means no limit.
}

// ServeDNS
// If entry returns an error, a SERVFAIL response will be sent back to client.
// If concurrentLimit is reached, the query will block and wait available token until ctx is done.
func (h *DefaultHandler) ServeDNS(ctx context.Context, qCtx *handler.Context, w ResponseWriter) {
	h.initOnce.Do(func() {
		if h.Logger != nil {
			h.logger = h.Logger
		} else {
			h.logger = zap.NewNop()
		}
		if h.ConcurrentLimit > 0 {
			h.limiter = concurrent_limiter.NewConcurrentLimiter(h.ConcurrentLimit)
		}
	})

	if h.limiter != nil {
		if int(h.limiter.Running()) > h.limiter.Max() { // too many waiting query, silently drop it.
			return
		}

		select {
		case h.limiter.Wait() <- struct{}{}:
			defer h.limiter.Done()
		case <-ctx.Done():
			return // ctx timeout, silently drop it.
		}
	}

	err := handler.ExecChainNode(ctx, qCtx, h.Entry)
	if err != nil {
		h.logger.Warn("entry returned an err", qCtx.InfoField(), zap.Error(err))
	} else {
		h.logger.Debug("entry returned", qCtx.InfoField(), zap.Stringer("status", qCtx.Status()))
	}

	var r *dns.Msg
	if err != nil || qCtx.Status() == handler.ContextStatusServerFailed {
		r = new(dns.Msg)
		r.SetReply(qCtx.Q())
		r.Rcode = dns.RcodeServerFailure
	} else {
		r = qCtx.R()
	}

	if r != nil {
		if h.RecursionAvailable {
			r.RecursionAvailable = true
		}
		if _, err := w.Write(r); err != nil {
			h.logger.Warn("write response", qCtx.InfoField(), zap.Error(err))
		}
	}
}

type DummyServerHandler struct {
	T       *testing.T
	WantMsg *dns.Msg
	WantErr error
}

func (d *DummyServerHandler) ServeDNS(_ context.Context, qCtx *handler.Context, w ResponseWriter) {
	var r *dns.Msg
	if d.WantMsg != nil {
		r = d.WantMsg.Copy()
		r.Id = qCtx.Q().Id
	} else {
		r = new(dns.Msg)
		r.SetReply(qCtx.Q())
	}

	_, err := w.Write(r)
	if err != nil {
		d.T.Errorf("DummyServerHandler: %v", err)
	}
}
