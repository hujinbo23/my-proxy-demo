package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

var (
	local_addr = "localhost:8082"
	relay_addr = "127.0.0.1:3333"
)

func init() {

	flag.StringVar(&local_addr, "l", "localhost:8082", "the local port")
	flag.StringVar(&relay_addr, "r", "127.0.0.1:3333", "remote server port")
}

func main() {
	flag.Parse()

	defer func() {
		err := recover()
		if err != nil {
			fmt.Println(err)
		}
	}()

	for {
		err := dialRelay()
		if err != nil {
			log.Println(err)
		}
	}
}

func dialRelay() error {

	relay_tcp_addr, err := net.ResolveTCPAddr("tcp", relay_addr)
	if err != nil {
		return err
	}

	relay_conn, err := net.DialTCP("tcp", nil, relay_tcp_addr)
	if err != nil {
		return err
	}
	fmt.Println("[连接 server 成功]", relay_conn.RemoteAddr())
	// TODO 认证流程
	var buf [64]byte

	for {
		relay_conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, err = io.ReadFull(relay_conn, buf[:4])
		if err != nil {
			relay_conn.Close()
			return err
		}
		relay_conn.SetReadDeadline(time.Time{})

		if bytes.Equal(buf[:4], []byte("PING")) {
			_, err = relay_conn.Write([]byte("PONG"))
			if err != nil {
				relay_conn.Close()
				return err
			}
		} else if bytes.Equal(buf[:4], []byte("CONN")) {
			go acceptConn(relay_conn, local_addr)
			return nil
		}
	}

}

func acceptConn(relay_conn *net.TCPConn, local_addr string) {

	var addr_len [2]byte
	_, err := io.ReadFull(relay_conn, addr_len[:2])
	if err != nil {
		relay_conn.Close()
		log.Println(err)
		return
	}
	public_addr := make([]byte, (int(addr_len[0])<<8)|int(addr_len[1]))
	_, err = io.ReadFull(relay_conn, public_addr)
	if err != nil {
		relay_conn.Close()
		log.Println(err)
		return
	}

	log.Println("accept:", string(public_addr))
	_, err = relay_conn.Write([]byte("ACPT"))
	if err != nil {
		relay_conn.Close()
		log.Println(err)
		return
	}

	local_tcp_addr, err := net.ResolveTCPAddr("tcp", local_addr)
	if err != nil {
		relay_conn.Close()
		log.Println(err)
		return
	}

	local_conn, err := net.DialTCP("tcp", nil, local_tcp_addr)
	if err != nil {
		relay_conn.Close()
		log.Println(err)
		return
	}

	go copyTCPConn(local_conn, relay_conn)
	go copyTCPConn(relay_conn, local_conn)

}

func copyTCPConn(dst, src *net.TCPConn) {
	io.Copy(dst, src)
	src.CloseRead()
	dst.CloseWrite()
}
