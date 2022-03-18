package health

type simpleChecker struct {
	pinger func(addr, publicUrl string) error
}

func NewFunc(pinger func(addr, publicUrl string) error) *simpleChecker {
	return &simpleChecker{
		pinger: pinger,
	}
}

func (fw *simpleChecker) Ping(addr, publicUrl string) error {
	return fw.pinger(addr, publicUrl)
}
