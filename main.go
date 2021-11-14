// Command rsyslogd implements a simple remote syslog daemon for
// Ubiquiti's EdgeRouter line.
package main

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	defer notifyStopping()

	w := &lumberjack.Logger{
		Filename:   "/var/log/lan-syslogd/syslog.log",
		MaxSize:    500,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	logger := log.New(w, "# ", log.Ldate|log.Lmicroseconds)
	if err := run(logger); err != nil {
		panic(err)
	}
}

func run(w *log.Logger) error {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: 514,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	notifyReadiness()

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)

		select {
		case s := <-ch:
			log.Printf("got %s, exiting", s)
			conn.Close()
		default:
		}
	}()

	buf := make([]byte, 65_535)
	for i := 0; ; i++ {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			log.Printf("ReadFrom: %v", err)
			continue
		}
		w.Print(string(buf[:n]))
	}
	return nil
}

// Thanks, Caddy.

func notifyReadiness() error {
	val, ok := os.LookupEnv("NOTIFY_SOCKET")
	if !ok || val == "" {
		return nil
	}
	return sdNotify(val, "READY=1")
}

func notifyStopping() error {
	val, ok := os.LookupEnv("NOTIFY_SOCKET")
	if !ok || val == "" {
		return nil
	}
	return sdNotify(val, "STOPPING=1")
}

func sdNotify(path, payload string) error {
	addr := &net.UnixAddr{
		Name: path,
		Net:  "unixgram",
	}
	conn, err := net.DialUnix(addr.Net, nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = io.Copy(conn, strings.NewReader(payload))
	return err
}
