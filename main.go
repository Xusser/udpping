package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Xusser/udpping/internal/utils"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
)

var (
	host string = "0.0.0.0"
	port int    = 5555

	pingInterval int = 1000
	pingPktSize  int = 64

	serverMode bool = false

	traceMode bool = false
)

func init() {
	flag.IntVar(&pingInterval, "i", 1000, "Ping interval")
	flag.IntVar(&pingPktSize, "l", 64, "Ping packet size")
	flag.BoolVar(&serverMode, "s", false, "Run as server")
	flag.BoolVar(&traceMode, "v", false, "Log with trace level")

	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
		ForceColors:      true,
	})
	log.SetOutput(ansicolor.NewAnsiColorWriter(os.Stderr))
	log.SetLevel(log.InfoLevel)
}

func printHelp() {
	flag.Usage()
}

func main() {
	flag.Parse()

	if traceMode {
		log.SetLevel(log.TraceLevel)
		log.Trace("Logging with trace level")
	}

	var args []string = flag.Args()
	var err error = nil

	lArgs := len(args)
	if lArgs >= 1 {
		arg := args[0]
		host = arg
		log.Tracef("Using host:%s", host)
		// TODO: check whether host is valid or not
	}

	if lArgs >= 2 {
		arg := args[1]
		if port, err = strconv.Atoi(arg); err != nil {
			log.Fatalf("Invalid port value:%s", arg)
		} else if port < 1 || port > 65535 {
			log.Fatalf("Invalid port value:%d, expected integer between 1 and 65535", port)
		}
		log.Tracef("Using port:%d", port)
	}

	if serverMode {
		log.Info("Running as server")
		server()
	} else if lArgs <= 0 {
		log.Errorf("Unexpected args size:%d", len(args))
		printHelp()
		return
	} else if pingInterval < 10 {
		log.Fatalf("Invalid ping interval:%d, expected integer larger than 10", pingInterval)
	} else if pingPktSize < 64 || pingPktSize > 1500 {
		log.Fatalf("Invalid packet size value:%d, expected integer between 64 and 1500", pingPktSize)
	}
	client()
}

func server() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.WithError(err).Fatal("Fail to resolve address")
		return
	}

	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.WithError(err).Fatalf("Fail to listen at udp://%s", addr.String())
		return
	}

	log.Infof("Listening at udp://%s", c.LocalAddr().String())

	for {
		var buf = make([]byte, 1500)
		nr, srcAddr, err := c.ReadFromUDP(buf)
		if err != nil {
			if err == net.ErrClosed {
				return
			}
			continue
		}

		log.Trace(string(buf[:nr]))
		log.Infof("Request from %s: Size=%d", srcAddr.String(), nr)
		nw, err := c.WriteToUDP(buf[:nr], srcAddr)
		if err != nil {
			continue
		}

		if nw != nr {
			continue
		}
	}
}

func client() {
	var countTotal uint64
	var countReceived uint64
	var rttSum time.Duration
	var rttMin time.Duration = math.MaxInt64
	var rttMax time.Duration

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.WithError(err).Fatal("Fail to resolve address")
	}

	log.Infof("Pinging %s with %d bytes data", addr.String(), pingPktSize)
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.WithError(err).Fatal("Fail to dial to %s", addr.String())
	}

	ctx, stop := context.WithCancel(context.Background())

	recvCh := make(chan []byte, 1)
	ch := make(chan os.Signal, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		var buf = make([]byte, 1500)

		for {
			nr, err := c.Read(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					log.WithError(err).Debug("Fail to read")
					continue
				}
			}
			recvCh <- buf[:nr]
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			payload := utils.RandStringBytesMaskImprSrcUnsafe(pingPktSize)
			_, err := c.Write([]byte(payload))
			if err != nil {
				log.WithError(err).Error("Fail to send ping")
				continue
			}
			countTotal++
			startTs := time.Now()
			deadline := startTs.Add(time.Millisecond * time.Duration(pingInterval))

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(deadline)):
				log.Warn("Request timeout")
				continue
			case b := <-recvCh:
				if !bytes.Equal(b, payload) {
					log.Errorf("Unexpected payload")
					<-time.After(time.Until(deadline))
					continue
				} else {
					log.Trace("Recv ping response")
				}
			}

			countReceived++
			elapsed := time.Since(startTs)
			rttSum += elapsed
			if rttMax < elapsed {
				rttMax = elapsed
			}
			if rttMin > elapsed {
				rttMin = elapsed
			}
			log.Infof("Reply from %s: Size=%d, Elapsed=%.2fms ", c.RemoteAddr().String(), pingPktSize, float64(elapsed.Nanoseconds())/1000/1000)
			<-time.After(time.Until(deadline))
		}
	}()

	signal.Notify(ch, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT /*, syscall.SIGUSR1*/)
	s := <-ch
	stop()
	wg.Done()
	fmt.Println("")
	log.Tracef("Catch signal[%s]", s.String())

	log.Infof("%d packets transmitted, %d received, %.2f%% packet loss", countTotal, countReceived, float64(countTotal-countReceived)/float64(countTotal)*100.0)
	if countReceived > 0 {
		log.Infof("rtt min/avg/max = %.2f/%.2f/%.2f",
			float64(rttMin.Nanoseconds())/1000/1000,
			(float64(rttSum.Nanoseconds())/float64(countReceived))/1000/1000,
			float64(rttMax.Nanoseconds())/1000/1000)
	}
}
