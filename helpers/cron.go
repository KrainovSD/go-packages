package helpers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type CronJobScheduler struct {
	registeredJobs *registeredJobs
	logger         *slog.Logger
	wg             sync.WaitGroup
}

type CronJobSchedulerOptions struct {
	Logger *slog.Logger
}

type CronJob struct {
	name   string
	cancel context.CancelFunc
	ctx    context.Context
}

func CreateCronJobScheduler(o CronJobSchedulerOptions) *CronJobScheduler {
	return &CronJobScheduler{
		logger:         o.Logger,
		registeredJobs: &registeredJobs{jobs: make([]*CronJob, 0, 10), mutex: sync.Mutex{}},
		wg:             sync.WaitGroup{},
	}
}

func (s *CronJobScheduler) RegisterJob(work func() error, name string, dur time.Duration) (*CronJob, error) {
	var ctx, cancel = context.WithCancel(context.Background())
	var job = &CronJob{
		name:   name,
		ctx:    ctx,
		cancel: cancel,
	}
	var err error

	var start = time.Now()
	if err = work(); err != nil {
		s.logger.LogAttrs(context.Background(), slog.LevelError, "cron job", slog.String("message", fmt.Sprintf("first execute cron job %s", name)), slog.String("error", err.Error()))
		return job, fmt.Errorf("first execute cron job %s: %w", name, err)
	}
	s.registeredJobs.Put(job)
	s.logger.LogAttrs(context.Background(), slog.LevelInfo, "cron job", slog.String("message", fmt.Sprintf("executed cron job %s", name)), slog.Duration("duration", time.Since(start)))

	s.wg.Add(1)
	go func() {
		var ticker = time.NewTicker(dur)
		defer ticker.Stop()
		defer cancel()
		defer s.wg.Done()

	Loop:
		for {
			select {
			case <-ticker.C:
				var start = time.Now()
				if err = work(); err != nil {
					s.logger.LogAttrs(context.Background(), slog.LevelError, "cron job", slog.String("message", fmt.Sprintf("execute cron job %s", name)), slog.String("error", err.Error()))
				}
				s.logger.LogAttrs(context.Background(), slog.LevelInfo, "cron job", slog.String("message", fmt.Sprintf("executed cron job %s", name)), slog.Duration("duration", time.Since(start)))
			case <-ctx.Done():
				ticker.Stop()
				s.registeredJobs.Delete(job)
				s.logger.LogAttrs(context.Background(), slog.LevelInfo, "cron job", slog.String("message", fmt.Sprintf("stopped cron job %s", name)))
				break Loop
			}

		}
	}()

	return job, nil
}
func (s *CronJobScheduler) StopJobs() {
	for _, job := range s.registeredJobs.jobs {
		job.Stop()
	}
	s.wg.Wait()
}

func (j *CronJob) Stop() {
	j.cancel()
}

type registeredJobs struct {
	jobs  []*CronJob
	mutex sync.Mutex
}

func (r *registeredJobs) Put(job *CronJob) {
	r.mutex.Lock()
	r.jobs = append(r.jobs, job)
	r.mutex.Unlock()
}

func (r *registeredJobs) Delete(job *CronJob) {
	r.mutex.Lock()
	FilterMutable(&r.jobs, func(t **CronJob) bool {
		return *t == job
	})
	r.mutex.Unlock()
}
