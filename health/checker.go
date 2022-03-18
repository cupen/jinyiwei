package health

type Checker interface {
	Ping(addr, publicUrl string) error
}
