package logger

type Log interface {
	Debug(content string)
	Debugf(content string, args ...interface{})
	Info(content string)
	Infof(content string, args ...interface{})
	Error(content string)
	Errorf(content string, args ...interface{})
	Fatal(content string, args ...interface{})
	Fatalf(content string, args ...interface{})
}
