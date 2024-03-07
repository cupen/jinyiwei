package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cupen/xdisco"
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

	// clientFlags = flag.NewFlagSet("client", flag.ExitOnError)
	// serverFlags = flag.NewFlagSet("server", flag.ExitOnError)
	broker = flag.String("broker", "etcd", "etcd or k8s")
	port   = flag.Int("port", 0, "tcp port of echoserver")

	usename = flag.String("username", "", "client username")
)

func main() {
	log.Info("debug", zap.Any("args", os.Args))
	flag.CommandLine.Parse(os.Args[2:])
	subcmd := os.Args[1]
	switch subcmd {
	case "client":
		runClient(*broker, *usename)
	case "server":
		runServer(*broker, *port)
	default:
		log2.Warnf("invalid subcommand: \"%s\"", subcmd)
		os.Exit(127)
	}
}

func runClient(broker, username string) {
	bk := setupBorker(broker)
	svc := setupService(bk)

	log.Info("======= server list =======")
	svc.GetServerList().ForEach(func(k string, s *server.Server) {
		log2.Infof("id:%s  addr: %s", k, s.PrivateAddress("tcp"))
	})
	log.Info("===========================")

	fmt.Printf("!hello '%s'!\n", username)

	for {
		s := svc.GetServerList().Lookup(username)
		if s == nil {
			fmt.Println("no server found")
			return
		}
		addr := s.PrivateAddress("tcp")
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			panic(fmt.Errorf("dial failed: %w", err))
		}
		showPrompt := func() {
			fmt.Printf("$ [%s]: ", addr)
		}

		showPrompt()

		scanner := bufio.NewScanner(os.Stdin)

		c := NewPacketCodec(conn)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "quit" || text == "exit" {
				defer c.Close()
				break
			}
			if err := c.Write([]byte(text)); err != nil {
				log2.Warnf("write failed: %v", err)
				defer c.Close()
				break
			}
			resp, err := c.Read()
			if err != nil {
				log2.Warn("read failed", zap.Error(err))
				defer c.Close()
				break
			}
			fmt.Printf("%s\n", string(resp))
			showPrompt()
		}
	}
	os.Exit(0)
}

func runServer(broker string, port int) {
	bk := setupBorker(broker)
	// svc := setupService(bk)
	listen := fmt.Sprintf("0.0.0.0:%d", port)
	lis, err := net.Listen("tcp4", listen)
	if err != nil {
		log.Panic("listen failed", zap.Error(err))
		return
	}
	defer lis.Close()

	listen = lis.Addr().String()
	serverId := fakeUID()
	host := GetLocalIP()
	if host == "" {
		panic("get local ip failed")
	}
	s := server.NewServer(serverId, "usercase01", host)
	s.Ports["tcp"] = pickupPort(listen)

	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := bk.Start(c, s); err != nil {
		log.Panic("server start failed", zap.Error(err))
		return
	}
	// c, cancel := context.WithCancel(context.Background())

	var wg = sync.WaitGroup{}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		s := <-sig
		log.Info("signal received", zap.Stringer("signal", s))
		cancel()
		lis.Close()

		time.Sleep(200 * time.Millisecond)
		wg.Done()
	}()
	log.Info("echoserver started", zap.Stringer("listen", lis.Addr()))
	defer lis.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Warn("accept failed", zap.Error(err))
			break
		}
		go onClientConnect(c, conn)
	}
	log.Info("echoserver stopped")
	wg.Wait()
}

func setupBorker(which string) (bk brokermod.Broker) {
	var err error
	switch which {
	case "etcd":
		opts := etcd.DefaultOptions()
		opts.BaseKey = "/xdisco/example01"
		bk, err = etcd.New(opts)
	case "k8s":
		bk, err = k8s.New(map[string]string{})
	default:
		err = fmt.Errorf("invalid broker name: %s", which)
	}
	if err != nil {
		panic(fmt.Errorf("setup broker failed: %w", err))
	}
	return
}

func setupService(bk brokermod.Broker) *xdisco.Service {
	hc := healthChecker()
	svc := xdisco.NewService("usercase01", bk, hc)
	svc.OnChanged(func(s *xdisco.Service) {
		slist := s.GetServerList()
		_ = slist
		// log.Info("current serverlist", zap.Int("servers", slist.Size()))
	})
	if err := svc.Start(context.TODO()); err != nil {
		log.Panic("service watch failed", zap.Error(err))
		return nil
	}
	return svc
}

func healthChecker() server.Checker {
	var c *TcpCodec
	ensureConn := func(addr string, force bool) (*TcpCodec, error) {
		if c == nil || force {
			_conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			c = NewPacketCodec(_conn)
		}
		return c, nil
	}

	hchecker := health.Custom(func(s *server.Server) error {
		addr := s.PrivateAddress("tcp")
		conn, err := ensureConn(addr, false)
		if err != nil {
			return err
		}
		if err = conn.Write([]byte("health")); err != nil {
			conn, err = ensureConn(addr, true)
			if err != nil {
				log.Warn("reconnect failed", zap.Error(err))
				return err
			}
			err = conn.Write([]byte("health"))
		}
		if err != nil {
			log.Warn("unhealth ", zap.Error(err))
			return err
		}
		resp, err := conn.Read()
		if err != nil {
			log.Warn("unhealth ", zap.Error(err))
			return err
		}
		if string(resp) != "health" {
			log.Warn("unhealth ", zap.Error(err))
			return fmt.Errorf("invalid echoserver. resp:%s", resp)
		}
		return nil
	})
	return hchecker
}

func onClientConnect(c context.Context, conn net.Conn) {
	// auto close or timeout
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	log.Info("new client from " + clientAddr)

	r := NewPacketCodec(conn)
	for {
		select {
		case <-c.Done():
			log2.Infof("<%s>: stopped", clientAddr)
			return
		default:
			p, err := r.Read()
			if err != nil {
				break
			}
			input := strings.TrimSpace(string(p))
			args := strings.Split(input, " ")
			cmd := strings.TrimSpace(args[0])
			log2.Infof("%s: input = \"%s\"", clientAddr, input)
			switch cmd {
			case "q", "quit", "exit":
				r.Write([]byte("bye~"))
				return
			case "health":
				r.Write([]byte(input))
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
				r.Write([]byte(input))
			}
		}
	}
}

// just for demo
func fakeUID() string {
	arr := make([]byte, 4)
	rand.Read(arr)
	return strings.Trim(base64.URLEncoding.EncodeToString(arr), "=")
}

func pickupPort(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	_port, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}
	return _port
}

// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
