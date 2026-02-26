package range_watch

var (
	ErrWatchReceiveAlreadyRegistered = strErr(`WatchRecv Already Registered`)
	ErrWatchReceiveNotRegistered     = strErr(`WatchRecv Not Registered`)
)

type strErr string

func (e strErr) Error() string {
	return string(e)
}
