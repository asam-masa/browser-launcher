package chromewindowtracking

type changeHook interface {
	Notifications() <-chan struct{}
	Failures() <-chan error
	Close() error
}
