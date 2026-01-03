package scheduling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchedulerStartup(t *testing.T) {
	t.Parallel()

	scheduler, err := NewScheduler()
	require.NoError(t, err)
	require.Empty(t, scheduler.jobs, "Scheduler should have no registered jobs after creation")
}

func TestSchedulerUsage(t *testing.T) {
	t.Parallel()

	scheduler, err := NewScheduler()
	require.NoError(t, err)

	// Register the first job.
	firstJob := JobName("first_job")
	err = scheduler.RegisterJob(firstJob, "* * * * *", func(_ context.Context) error { return nil })
	require.NoError(t, err)
	require.Len(t, scheduler.jobs, 1)
	require.Contains(t, scheduler.jobs, firstJob)

	// Register the second job.
	secondJob := JobName("second_job")
	err = scheduler.RegisterJob(secondJob, "*/5 * * * *", func(_ context.Context) error { return nil })
	require.NoError(t, err)
	require.Len(t, scheduler.jobs, 2)
	require.Contains(t, scheduler.jobs, secondJob)

	// Update the first job.
	err = scheduler.RegisterJob(firstJob, "0 2 * * 1", func(_ context.Context) error { return nil })
	require.NoError(t, err)
	require.Len(t, scheduler.jobs, 2)
	require.Contains(t, scheduler.jobs, firstJob)
}

func TestCrontabValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		crontab  string
		expected error
	}{
		{
			name:     "Valid standard cron",
			crontab:  "0 0 * * *",
			expected: nil,
		},
		{
			name:     "Too few fields",
			crontab:  "0 0 * *",
			expected: ErrInvalidCronTab,
		},
		{
			name:     "Too many fields",
			crontab:  "0 0 * * * *",
			expected: ErrInvalidCronTab,
		},
		{
			name:     "Non-numeric characters",
			crontab:  "a b c d e",
			expected: ErrInvalidCronTab,
		},
		{
			name:     "Empty string",
			crontab:  "",
			expected: ErrInvalidCronTab,
		},
		{
			name:     "Only whitespace",
			crontab:  "     ",
			expected: ErrInvalidCronTab,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheduler, err := NewScheduler()
			require.NoError(t, err)

			got := scheduler.RegisterJob(JobName("test"), tc.crontab, func(_ context.Context) error { return nil })
			require.Equal(t, tc.expected, got, tc.name)
		})
	}
}
