package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
)

// TODO 改成 flag 形式
var (
	// 本地需要暴露的服务端口
	localServerAddr = "127.0.0.1:8082"

	remoteIP = "192.168.124.16"
	// 远端的服务控制通道，用来传递控制信息，如出现新连接和心跳
	remoteControlAddr = remoteIP + ":8888"
	// 远端服务端口，用来建立隧道
	remoteServerAddr = remoteIP + ":8889"
)

const (
	KeepAlive     = "KEEP_ALIVE"
	NewConnection = "NEW_CONNECTION"
)

func CreateTCPConn(addr string) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	tcpListener, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return tcpListener, nil
}

func copy2Conn(local *net.TCPConn, remote *net.TCPConn) {
	go copyConn(local, remote)
	go copyConn(remote, local)
}

func copyConn(local *net.TCPConn, remote *net.TCPConn) {
	defer local.Close()
	defer remote.Close()

	_, err := io.Copy(local, remote)
	if err != nil {
		log.Println("[copy failed", err.Error())
		return
	}
}

func main() {
	fmt.Println("hello my-proxy client")
	tcpConn, err := CreateTCPConn(remoteControlAddr)
	if err != nil {
		log.Println("[连接失败]:", remoteControlAddr)
		return
	}
	log.Println("[连接成功]:", remoteControlAddr)

	reader := bufio.NewReader(tcpConn)
	for {
		s, err := reader.ReadString('\n')
		if err != nil || err == io.EOF {
			break
		}

		// 当有新连接信号出现时，新建立一个tcp连接
		if s == NewConnection+"\n" {
			go connectionLocalAndRemote()
		}
	}
	log.Println("[已断开]:", remoteControlAddr)
}

func connectionLocalAndRemote() {
	local := connectLocal()
	remote := connectRemote()

	if local != nil && remote != nil {
		copy2Conn(local, remote)
	} else {
		if local != nil {
			_ = local.Close()
		}
		if remote != nil {
			_ = remote.Close()
		}
	}

}

func connectLocal() *net.TCPConn {
	conn, err := CreateTCPConn(localServerAddr)
	if err != nil {
		log.Println("[连接本地服务失败]: ", err.Error())
	}
	return conn
}

func connectRemote() *net.TCPConn {
	conn, err := CreateTCPConn(remoteServerAddr)
	if err != nil {
		log.Println("[连接远程服务失败]: ", err.Error())
	}
	return conn
}
