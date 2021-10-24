package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	quic "github.com/lucas-clemente/quic-go"
	"golang.org/x/sync/semaphore"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	VersionDoQ00 = "doq-i00"
	VersionDoQ01 = "doq-i01"
	VersionDoQ02 = "doq-i02"
	VersionDoQ03 = "doq-i03"
	VersionDoQ04 = "doq-i04"
	VersionDoQ05 = "doq-i05"
	VersionDoQ06 = "doq-i06"
)

var DefaultDoQVersions = []string{VersionDoQ06, VersionDoQ05, VersionDoQ04, VersionDoQ03, VersionDoQ02, VersionDoQ01, VersionDoQ00}

var DefaultQUICVersions = []quic.VersionNumber{
	quic.Version1,
	quic.VersionDraft34,
	quic.VersionDraft32,
	quic.VersionDraft29,
}

var port853Flag = flag.Bool("port853", false, "verify on port 853")

func establishConnection(ip net.IP) bool {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos: DefaultDoQVersions,
	}

	quicConf := &quic.Config{
		HandshakeIdleTimeout: time.Second * 2,
		Versions: DefaultQUICVersions,
	}

	var ports []string
	if *port853Flag {
		ports = []string{"853"}
	} else {
		ports = []string{"784", "8853"}
	}

	reachable := make(chan bool)
	go func() {
		for _, port := range ports {
			session, err := quic.DialAddr(ip.String() + ":" + port, tlsConf, quicConf)
			if err != nil {
				continue
			}
			reachable <- true
			session.CloseWithError(0, "")
			return
		}

		reachable <- false
	}()

	return <- reachable
}


func main() {
	parallelLimit := flag.Int("parallel", 30, "sets the limit for parallel processes")

	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		println("need 2 arguments: [in file] [out file]")
		os.Exit(1)
	}

	inFile, err := os.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}
	defer inFile.Close()

	outFile, err := os.OpenFile(args[1], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	var sem = semaphore.NewWeighted(int64(*parallelLimit))

	var wg sync.WaitGroup

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		ip := net.ParseIP(scanner.Text())
		if ip.To4() != nil {
			sem.Acquire(context.Background(), 1)
			wg.Add(1)
			go func() {
				reachable := establishConnection(ip)
				if reachable {
					if _, err := outFile.WriteString(ip.String() + "\n"); err != nil {
						log.Println(err)
					}
				}
				sem.Release(1)
				wg.Done()
			}()
		}
	}

	wg.Wait()
}
