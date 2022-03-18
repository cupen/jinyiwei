package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	sd "github.com/cupen/xdisco"
	brokermod "github.com/cupen/xdisco/broker"
	"github.com/cupen/xdisco/broker/etcd"
	"github.com/cupen/xdisco/broker/k8s"
	"github.com/cupen/xdisco/health"
	"github.com/cupen/xdisco/logs"
	"github.com/cupen/xdisco/server"
	"go.uber.org/zap"
)

var (
	log  = logs.Logger("info")
	log2 = log.Sugar()

	broker = flag.String("broker", "etcd", "etcd or k8s")
	port   = flag.Int("port", 0, "tcp port of echoserver")
)

func main() {
	flag.Parse()
	var bk brokermod.Broker
	var err error
	switch *broker {
	case "etcd":
		bk, err = etcd.New("/xdisco/examples", 3*time.Second)
	case "k8s":
		bk, err = k8s.New(map[string]string{})
	default:
		log.Fatal("invalid broker", zap.String("broker", *broker))
	}

	if err != nil {
		log.Fatal("broker init failed", zap.Error(err))
	}

	hc := echoServerHealth()
	svc := sd.NewService("usercase01", bk, hc)
	svc.OnChanged(func(s *sd.Service) {
		slist := s.GetServerList()
		log.Info("current serverlist", zap.Int("servers", slist.Size()))
	})

	if err := svc.Start(context.TODO()); err != nil {
		log.Panic("service watch failed", zap.Error(err))
		return
	}

	listen := fmt.Sprintf("0.0.0.0:%d", *port)
	startEchoServer(&listen)

	id := listen
	s := server.NewServer(id, "usercase01", listen)
	if err := bk.Start(context.TODO(), s); err != nil {
		log.Panic("server start failed", zap.Error(err))
		return
	}
	select {}
}

func startEchoServer(listen *string) {
	l, err := net.Listen("tcp4", *listen)
	if err != nil {
		log.Panic("listen failed", zap.Error(err))
		return
	}
	*listen = l.Addr().String()
	go func() {
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}
			go func() {
				// auto close or timeout
				defer c.Close()
				log.Info("new client from " + c.RemoteAddr().String())
				clientAddr := c.RemoteAddr().String()
				running := true
				for running {
					c.SetDeadline(time.Now().Add(10 * time.Second))
					inputData, err := bufio.NewReader(c).ReadString('\n')
					if err != nil {
						if err != io.EOF {
							log.Warn("invalid client: read failed ", zap.Error(err))
						}
						break
					}
					input := strings.TrimSpace(string(inputData))
					args := strings.Split(input, " ")
					cmd := strings.TrimSpace(args[0])
					log2.Infof("%s: input <%s>", clientAddr, input)
					switch cmd {
					case "quit", "exit":
						c.Write([]byte("quited" + "\n"))
						running = false
					case "health":
						c.Write([]byte(input + "\n"))
					case "connect":
						if len(args) <= 1 {
							log.Warn("invalid input")
							continue
						}
						log2.Infof("%s connecting %s", clientAddr, args[1])
						conn, err := net.Dial("tcp", args[1])
						if err != nil {
							log2.Infof("%s connecting %s failed: err:%v", clientAddr, args[1], err)
						} else {
							log2.Infof("%s connected %s", clientAddr, args[1])
							conn.Close()
						}
					default:
						c.Write([]byte(input + "\n"))
					}
				}
			}()
		}
	}()
	log.Info("echoserver started", zap.String("listen", *listen))
}

func echoServerHealth() health.Checker {
	var client net.Conn
	ensureConn := func(addr string, force bool) (net.Conn, error) {
		if client == nil || force {
			_conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			client = _conn
		}
		return client, nil
	}

	hchecker := health.NewFunc(func(addr, publicAddr string) error {
		conn, err := ensureConn(addr, false)
		if err != nil {
			return err
		}
		if _, err = conn.Write([]byte("health\n")); err != nil {
			conn, err = ensureConn(addr, true)
			if err != nil {
				return err
			}
			_, err = conn.Write([]byte("health\n"))

		}
		if err != nil {
			return err
		}
		respData, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return err
		}
		resp := strings.TrimSpace(string(respData))
		if resp != "health" {
			return fmt.Errorf("invalid echoserver. resp:%s", resp)
		}
		return nil
	})
	return hchecker
}
