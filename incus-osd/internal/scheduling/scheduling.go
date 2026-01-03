package scheduling

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// JobName represents the name of a periodic job.
type JobName string

// Scheduler represents a background job scheduler.
type Scheduler struct {
	jobs      map[JobName]uuid.UUID
	scheduler gocron.Scheduler
}

// JobFunc represents the type of function that executes a scheduled job.
type JobFunc func(context.Context) error

// ErrInvalidCronTab is returned when an invalid crontab expression is provided.
var ErrInvalidCronTab error = errors.New("invalid crontab expression")

// NewScheduler creates a new Scheduler.
func NewScheduler() (Scheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return Scheduler{}, err
	}

	return Scheduler{
		jobs:      map[JobName]uuid.UUID{},
		scheduler: scheduler,
	}, nil
}

// RegisterJob registers a job in the Scheduler.
//
// If the job does not exist, it is created. If it already exists, it is updated.
func (s *Scheduler) RegisterJob(name JobName, crontab string, jobFunc JobFunc) error {
	cron := gocron.NewDefaultCron(false)

	// Validate scrub schedule expression.
	err := cron.IsValid(crontab, time.UTC, time.Now())
	if err != nil {
		return ErrInvalidCronTab
	}

	id, ok := s.jobs[name]
	if ok {
		_, err := s.scheduler.Update(id,
			gocron.CronJob(crontab, false),
			gocron.NewTask(wrapJob(name, jobFunc)),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if err != nil {
			return err
		}
	} else {
		job, err := s.scheduler.NewJob(
			gocron.CronJob(crontab, false),
			gocron.NewTask(wrapJob(name, jobFunc)),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if err != nil {
			return err
		}

		s.jobs[name] = job.ID()
	}

	return nil
}

// Start starts the scheduler and its registered jobs.
func (s *Scheduler) Start() {
	s.scheduler.Start()
}

// Shutdown shuts down the scheduler and its registered jobs.
func (s *Scheduler) Shutdown() error {
	return s.scheduler.Shutdown()
}

func wrapJob(name JobName, jobFunc JobFunc) func(context.Context) {
	return func(ctx context.Context) {
		select {
		// If the context is already cancelled, don't start the job.
		case <-ctx.Done():
			return

		default:
			slog.InfoContext(ctx, "Executing periodic job", slog.String("job", string(name)))

			err := jobFunc(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "Error running periodic job", slog.String("job", string(name)), slog.Any("error", err))
			}
		}
	}
}
