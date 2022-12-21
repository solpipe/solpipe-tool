package util

type Base interface {
	CloseSignal() <-chan error
	Close() error
}
