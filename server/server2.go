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
	localPort  int
	remotePort int
)

func init() {
	flag.IntVar(&localPort, "l", 5002, "the user link port")
	flag.IntVar(&remotePort, "r", 3333, "client listen port")
}

func main() {
	flag.Parse()

	defer func() {
		err := recover()
		if err != nil {
			fmt.Println(err)
		}
	}()

	remote_addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", remotePort))
	if err != nil {
		log.Fatalln(err)
	}

	public_addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		log.Fatalln(err)
	}

	remote_listener, err := net.ListenTCP("tcp", remote_addr)
	if err != nil {
		log.Fatalln(err)
	}

	public_listener, err := net.ListenTCP("tcp", public_addr)
	if err != nil {
		log.Fatalln(err)
	}

	public_conn_chan := make(chan *net.TCPConn)

	// 监听用户连接请求
	go acceptUserConn(public_listener, public_conn_chan)

	// 监听客户端请求
	for {
		relay_conn, err := remote_listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			// TODO Sleep a time
		} else {
			go acceptClientConn(relay_conn, public_conn_chan)
			go keepalive(relay_conn)
		}
	}

}

func acceptUserConn(public_listener *net.TCPListener, public_conn_chan chan *net.TCPConn) {

	for {
		public_conn, err := public_listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			// TODO Sleep a time
		} else {
			public_conn_chan <- public_conn
		}

	}
}

func acceptClientConn(relay_conn *net.TCPConn, public_conn_chan chan *net.TCPConn) {
	// var buf [64]byte
	// 认证流程

	// 转发user请求数据
	for {
		select {
		case public_conn := <-public_conn_chan:
			public_addr := public_conn.RemoteAddr().String()
			log.Println("accept:", public_addr)
			buf := []byte{'C', 'O', 'N', 'N', uint8(len(public_addr) >> 8), uint8(len(public_addr))}
			buf = append(buf, public_addr...)
			_, err := relay_conn.Write(buf)
			if err != nil {
				log.Println(err)
				relay_conn.Close()
				public_conn_chan <- public_conn
				return
			}

			for {
				_, err = io.ReadFull(relay_conn, buf[:4])
				if err != nil {
					log.Println(err)
					relay_conn.Close()
					public_conn_chan <- public_conn
					return
				}

				if bytes.Equal(buf[:4], []byte("ACPT")) {
					break
				}
			}

			// 开始反向代理传输
			go copyTCPConn(relay_conn, public_conn)
			go copyTCPConn(public_conn, relay_conn)
			return
			// case <-time.After(60 * time.Second):
			// 	_, err := relay_conn.Write([]byte("PING"))
			// 	if err != nil {
			// 		log.Println(err)
			// 		relay_conn.Close()
			// 		return
			// 	}

			// 	for {
			// 		relay_conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			// 		_, err = io.ReadFull(relay_conn, buf[:4])
			// 		if err != nil {
			// 			log.Println(err)
			// 			relay_conn.Close()
			// 			return
			// 		}
			// 		relay_conn.SetReadDeadline(time.Time{})

			// 		if bytes.Equal(buf[:4], []byte("PONG")) {
			// 			break
			// 		}
			// 	}
		}
	}
}

// 循环读 直到 ACCP，表示客户端已经准备好了，可以开始可以建立反向代理
func waitClientAccp(relay_conn, public_conn *net.TCPConn, public_conn_chan chan *net.TCPConn, buf []byte) {
	for {
		_, err := io.ReadFull(relay_conn, buf[:4])

		if err != nil {
			log.Println(err)
			relay_conn.Close()
			public_conn_chan <- public_conn
			return
		}

		if bytes.Equal(buf[:4], []byte("ACPT")) {
			break
		}
	}
}

/*
*
@param dst 表示目标（Destination）的缩写，表示数据传输或复制的目标位置或变量
@param src 表示源（Source）的缩写，表示数据传输或复制的源位置或变量。
*/
func copyTCPConn(dst, src *net.TCPConn) {
	io.Copy(dst, src)
	src.CloseRead()
	dst.CloseWrite()
}

func keepalive(relay_conn *net.TCPConn) {
	go func() {
		for {
			if relay_conn == nil {
				return
			}
			_, err := relay_conn.Write([]byte("PING"))
			if err != nil {
				log.Println("[已断开客户端连接] ", relay_conn.RemoteAddr())
				return
			}
			var buf [4]byte
			for {
				relay_conn.SetReadDeadline(time.Now().Add(90 * time.Second))
				_, err = io.ReadFull(relay_conn, buf[:4])
				if err != nil {
					log.Println(err)
					relay_conn.Close()
					return
				}
				relay_conn.SetReadDeadline(time.Time{})

				if bytes.Equal(buf[:4], []byte("PONG")) {
					break
				}
			}
			time.Sleep(time.Second * 60)
		}
	}()
}
