// SPDX-License-Identifier: MPL-2.0
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reconciler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Reconciler interface {
	Start() // Start the reconciliation loop
	Stop()  // Stop the reconciliation loop
}

type Impl interface {
	Reconcile(ctx context.Context) error
}

type reconciler struct {
	ctx      context.Context // Original, passed-in context
	cancel   func()
	running  bool
	mutex    sync.Mutex
	wg       sync.WaitGroup
	log      *zerolog.Logger
	interval time.Duration
	impl     Impl
}

func New(ctx context.Context, logger *zerolog.Logger, interval time.Duration, impl Impl) (Reconciler, error) {
	if logger == nil {
		return nil, errors.New("must specify logger")
	}

	if interval == 0 {
		return nil, errors.New("must specify reconciliation interval")
	}

	return &reconciler{
		ctx:      ctx,
		log:      logger,
		interval: interval,
		impl:     impl,
	}, nil
}

func (r *reconciler) Start() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.running {
		return // No-op if already running
	}

	var ctx context.Context
	ctx, r.cancel = context.WithCancel(r.ctx)
	r.running = true
	r.wg.Add(1)

	go func() {
		r.log.Debug().Msg("starting reconciliation")
		defer r.log.Debug().Msg("stopping reconciliation")
		ticker := time.NewTicker(r.interval)

		for {
			r.log.Debug().Msg("performing reconciliation")
			err := r.impl.Reconcile(ctx)
			if errors.Is(err, context.Canceled) {
				return // Context cancellation is expected if stopped
			}

			if err != nil {
				r.log.Err(err).Msg("reconciliation failed")
			} else {
				r.log.Debug().Msg("reconciliation finished")
			}

			select {
			case <-ctx.Done():
				r.wg.Done()
				return
			case <-ticker.C:
			}
		}
	}()
}

func (r *reconciler) Stop() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.cancel()
	r.running = false
	r.wg.Wait()
}
