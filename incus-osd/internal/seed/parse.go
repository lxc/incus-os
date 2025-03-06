package seed

import (
	"errors"
)

// ErrNoSeedPartition is returned when the seed partition couldn't be found.
var ErrNoSeedPartition = errors.New("no seed partition could be found")

// ErrNoSeedData is returned when a partition could be found but no data was found in it.
var ErrNoSeedData = errors.New("no seed data present in the partition")

// ErrNoSeedSection is returned when the seed data is available but the requested section/file couldn't be found.
var ErrNoSeedSection = errors.New("requested seed section couldn't be found")

const seedPartitionPath = "/dev/disk/by-partlabel/seed-data"
