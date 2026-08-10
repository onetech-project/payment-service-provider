package worker

import (
	"context"
	"log"

	"backbone-new/internal/domain"

	"github.com/hibiken/asynq"
)

// TaskVAStatusReconcile is the periodic sweep that asks the vendor about
// transactions this service still believes are pending
// (feature 014-vendor-status-reconciliation).
const TaskVAStatusReconcile = "va:status:reconcile"

// ReconcileWorker runs the sweep on the schedule an asynq.Scheduler enqueues.
//
// Going through the queue rather than a bare in-process ticker is deliberate:
// the scheduler enqueues one task per interval regardless of how many replicas
// are running, so a horizontally-scaled deployment does not aim N concurrent
// sweeps — and N times the outbound traffic — at the vendor.
type ReconcileWorker struct {
	reconciler domain.VAStatusReconciler
}

// NewReconcileWorker creates the sweep worker.
func NewReconcileWorker(reconciler domain.VAStatusReconciler) *ReconcileWorker {
	return &ReconcileWorker{reconciler: reconciler}
}

// HandleReconcileSweep runs one sweep.
//
// A failure to SELECT candidates is returned so asynq retries it; per-record
// failures are already swallowed inside Sweep, because one unreachable
// transaction must not stop the rest from being recovered.
func (w *ReconcileWorker) HandleReconcileSweep(ctx context.Context, _ *asynq.Task) error {
	results, err := w.reconciler.Sweep(ctx)
	if err != nil {
		log.Printf("event=va_reconcile_sweep_failed err=%v", err)
		return err
	}

	settled := 0
	for _, r := range results {
		if r.Settled {
			settled++
		}
	}
	if len(results) > 0 {
		log.Printf("event=va_reconcile_sweep_result examined=%d settled=%d", len(results), settled)
	}
	return nil
}

// RegisterReconcileWorker wires the sweep handler into the asynq mux.
func RegisterReconcileWorker(mux *asynq.ServeMux, worker *ReconcileWorker) {
	mux.HandleFunc(TaskVAStatusReconcile, worker.HandleReconcileSweep)
}
