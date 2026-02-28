package wazero_range_watch

import (
	"fmt"
)

var (
	ErrWatchNotFound    = fmt.Errorf(`Watch not found`)
	ErrWatchExists      = fmt.Errorf(`Watch exists`)
	ErrWatchClosed      = fmt.Errorf(`Watch closed`)
	ErrWatchAlreadyOpen = fmt.Errorf(`Watch already open`)
)
