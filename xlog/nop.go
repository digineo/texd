package xlog

type nop struct{}

func NewNop() Logger {
	l, _ := New(TypeNop, nil, nil)
	return l
}

func (*nop) Debug(msg string, args ...any) {}
func (*nop) Info(msg string, args ...any)  {}
func (*nop) Warn(msg string, args ...any)  {}
func (*nop) Error(msg string, args ...any) {}
func (*nop) Fatal(msg string, args ...any) { /* XXX: os.Exit? */ }
func (n *nop) With(args ...any) Logger     { return n }
