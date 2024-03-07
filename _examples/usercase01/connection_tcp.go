package main

import (
	"encoding/binary"
	"net"
	"sync"
	"time"
)

type TcpCodec struct {
	conn net.Conn
	pool sync.Pool
}

func NewPacketCodec(conn net.Conn) *TcpCodec {
	return &TcpCodec{
		conn: conn,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 512)
			},
		},
	}
}

func (s *TcpCodec) Read() ([]byte, error) {
	s.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var head = [2]byte{0, 0}
	_, err := s.conn.Read(head[:])
	if err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint16(head[:])
	// s.pool.Get()
	var body = make([]byte, int(size))
	if _, err := s.conn.Read(body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *TcpCodec) Write(data []byte) error {
	s.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	var head = [2]byte{}
	binary.LittleEndian.PutUint16(head[:], uint16(len(data)))
	s.conn.Write(head[:])
	_, err := s.conn.Write(data)
	return err
}

func (s *TcpCodec) Close() error {
	return s.conn.Close()
}
