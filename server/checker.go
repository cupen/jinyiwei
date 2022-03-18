package server

type Checker interface {
	Ping(*Server) error
}
