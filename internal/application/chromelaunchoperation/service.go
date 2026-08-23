package chromelaunchoperation

import (
	"context"
	"errors"
	"sync"
	"time"

	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

const defaultOperationTimeout = 60 * time.Second

var ErrServiceNotConfigured = errors.New("chrome launch operation service is not configured")

var ErrServiceShuttingDown = errors.New("chrome launch operation service is shutting down")

type WorkflowRunner interface {
	Run(context.Context, launchcondition.Input) WorkflowResult
}

type StateNotifier interface {
	Notify(Snapshot)
}

type Service struct {
	workflow  WorkflowRunner
	registry  *Registry
	timeout   time.Duration
	notifier  StateNotifier
	notifyMu  *sync.Mutex
	lifecycle *serviceLifecycle
}

type serviceLifecycle struct {
	mu           sync.Mutex
	shuttingDown bool
	wg           sync.WaitGroup
}

func NewService(workflow WorkflowRunner, registry *Registry, notifier ...StateNotifier) Service {
	var stateNotifier StateNotifier
	if len(notifier) > 0 {
		stateNotifier = notifier[0]
	}
	return Service{
		workflow:  workflow,
		registry:  registry,
		timeout:   defaultOperationTimeout,
		notifier:  stateNotifier,
		notifyMu:  &sync.Mutex{},
		lifecycle: &serviceLifecycle{},
	}
}

func (s Service) Start(parent context.Context, input launchcondition.Input) (Snapshot, error) {
	if parent == nil {
		return Snapshot{}, ErrNilContext
	}
	if s.workflow == nil || s.registry == nil || s.timeout <= 0 || s.notifyMu == nil || s.lifecycle == nil {
		return Snapshot{}, ErrServiceNotConfigured
	}

	timeoutContext, stopTimeout := context.WithTimeout(parent, s.timeout)
	s.lifecycle.mu.Lock()
	if s.lifecycle.shuttingDown {
		s.lifecycle.mu.Unlock()
		stopTimeout()
		return Snapshot{}, ErrServiceShuttingDown
	}
	s.lifecycle.wg.Add(1)
	snapshot, operationContext, err := s.registry.Start(timeoutContext)
	if err != nil {
		s.lifecycle.wg.Done()
		s.lifecycle.mu.Unlock()
		stopTimeout()
		return Snapshot{}, err
	}
	s.lifecycle.mu.Unlock()

	s.notify(snapshot)
	go func() {
		defer s.lifecycle.wg.Done()
		s.run(snapshot.ID, operationContext, stopTimeout, input)
	}()
	return snapshot, nil
}

func (s Service) Cancel(id ID) (CancelStatus, Snapshot) {
	if s.registry == nil {
		return CancelNotFound, Snapshot{}
	}
	status, snapshot := s.registry.Cancel(id)
	if status == CancelAccepted {
		s.notify(snapshot)
	}
	return status, snapshot
}

func (s Service) Get(id ID) (Snapshot, bool) {
	if s.registry == nil {
		return Snapshot{}, false
	}
	return s.registry.Get(id)
}

func (s Service) Current() (Snapshot, bool) {
	if s.registry == nil {
		return Snapshot{}, false
	}
	return s.registry.Current()
}

func (s Service) wait(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if s.lifecycle == nil {
		return ErrServiceNotConfigured
	}
	done := make(chan struct{})
	go func() {
		s.lifecycle.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s Service) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if s.registry == nil || s.lifecycle == nil {
		return ErrServiceNotConfigured
	}

	s.lifecycle.mu.Lock()
	s.lifecycle.shuttingDown = true
	if snapshot, ok := s.registry.Current(); ok {
		s.Cancel(snapshot.ID)
	}
	s.lifecycle.mu.Unlock()
	return s.wait(ctx)
}

func (s Service) run(
	id ID,
	ctx context.Context,
	stopTimeout context.CancelFunc,
	input launchcondition.Input,
) {
	defer stopTimeout()

	running, err := s.registry.Transition(id, StateRunning)
	if err != nil {
		finished, finishErr := s.registry.Finish(id, StateCancelled, ErrorCodeNone)
		if finishErr == nil {
			s.notify(finished)
		}
		return
	}
	s.notify(running)

	result := make(chan WorkflowResult, 1)
	s.lifecycle.wg.Add(1)
	go func() {
		defer s.lifecycle.wg.Done()
		defer func() {
			if recover() != nil {
				result <- WorkflowResult{
					Outcome:   OutcomeFailed,
					ErrorCode: ErrorCodeUnexpected,
				}
			}
		}()
		result <- s.workflow.Run(ctx, input)
	}()

	select {
	case workflowResult := <-result:
		terminal, code := terminalSnapshot(workflowResult)
		finished, err := s.registry.Finish(id, terminal, code)
		if err == nil {
			s.notify(finished)
		}
	case <-ctx.Done():
		terminal := StateCancelled
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			terminal = StateTimedOut
		}
		finished, err := s.registry.Finish(id, terminal, ErrorCodeNone)
		if err == nil {
			s.notify(finished)
		}
	}
}

func (s Service) notify(snapshot Snapshot) {
	if s.notifier == nil {
		return
	}
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()

	current, ok := s.registry.Get(snapshot.ID)
	if ok && current == snapshot {
		s.notifier.Notify(snapshot)
	}
}

func terminalSnapshot(result WorkflowResult) (State, ErrorCode) {
	switch result.Outcome {
	case OutcomeCompleted:
		return StateCompleted, ErrorCodeNone
	case OutcomeCancelled:
		return StateCancelled, ErrorCodeNone
	case OutcomeTimedOut:
		return StateTimedOut, ErrorCodeNone
	case OutcomeInvalid, OutcomeFailed:
		code := result.ErrorCode
		if code == ErrorCodeNone {
			code = ErrorCodeUnexpected
		}
		return StateFailed, code
	default:
		return StateFailed, ErrorCodeUnexpected
	}
}
