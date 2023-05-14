package main

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	KeepAlive      = "KEEP_ALIVE"
	NewConnecttion = "NEW_CONNECTION"

	controlAddr = "0.0.0.0:8888"
	tunnelAddr  = "0.0.0.0:8889"
	visitAddr   = "0.0.0.0:9999"
)

var (
	clientConn     *net.TCPConn
	connectionPool map[string]*ConnMatch
	lock           sync.Mutex
)

type ConnMatch struct {
	addTime time.Time
	accept  *net.TCPConn
}

func CreateTcpListener(addr string) (*net.TCPListener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}
	return tcpListener, nil
}

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

func copy2Conn(local, remote *net.TCPConn) {
	go copyConn(local, remote)
	go copyConn(remote, local)
}

func copyConn(local, remote *net.TCPConn) {
	defer local.Close()
	defer remote.Close()

	_, err := io.Copy(local, remote)
	if err != nil {
		log.Println("[copy failed", err.Error())
		return
	}
}

/**
1. 监听控制通道，接收客户端的连接请求
2. 监听访问端口，接收来自用户的 http 请求
3. 第二步接收到请求之后需要存放一下这个连接并同时发消息给客户端，告诉客户端有用户访问了，赶紧建立隧道进行通信
4. 监听隧道通道，接收来自客户端的连接请求，将客户端的连接与用户的连接建立起来（也是用工具方法
*/

func main() {

	connectionPool = make(map[string]*ConnMatch, 32)
	go createControlChannnel()
	go acceptUserRequest()
	go acceptClientRequest()

	cleanConnectionPool()
}

// 创建一个控制连接通道，用于传递控制消息，如心跳，创建新连接
func createControlChannnel() {
	tcpListener, err := CreateTcpListener(controlAddr)
	if err != nil {
		panic(err)
	}

	log.Println("[已监听]", controlAddr)
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("[新连接]: ", tcpConn.RemoteAddr().String())
		//如果当前已经有一个客户端存在，则丢弃这个连接
		if clientConn != nil {
			_ = tcpConn.Close()
		} else {
			clientConn = tcpConn
			go keepAlive()
		}
	}
}

// 和客户端保持心跳
func keepAlive() {
	go func() {
		for {
			if clientConn == nil {
				return
			}
			_, err := clientConn.Write(([]byte)("KEEP_ALIVE" + "\n"))
			if err != nil {
				log.Println("[已断开客户端连接] ", clientConn.RemoteAddr())
				clientConn = nil
				return
			}
			time.Sleep(time.Second * 3)
		}
	}()
}

// 监听来自用户的请求
func acceptUserRequest() {

	tcpListener, err := CreateTcpListener(visitAddr)
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close()
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			continue
		}
		log.Println("[新的用户端连接]: ", tcpConn.RemoteAddr().String())
		addConn2Pool(tcpConn)
		sendMessage("NEW_CONNECTION" + "\n")
	}
}

// 将用户来的连接放入连接池中
func addConn2Pool(accept *net.TCPConn) {
	lock.Lock()
	defer lock.Unlock()

	now := time.Now()
	connectionPool[strconv.FormatInt(now.UnixNano(), 10)] = &ConnMatch{now, accept}
}

// 发送给客户端新消息
func sendMessage(message string) {
	if clientConn == nil {
		log.Println("[无已连接的客户端]")
		return
	}
	_, err := clientConn.Write([]byte(message))
	if err != nil {
		log.Println("[发送消息异常]: message: ", message)
	}
}

// 接收客户端来的请求并建立隧道
func acceptClientRequest() {
	tcpListener, err := CreateTcpListener(tunnelAddr)
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close()

	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			continue
		}
		log.Println("[新的客户端连接]: ", tcpConn.RemoteAddr().String())
		go establishTunnel(tcpConn)
	}
}

func establishTunnel(tunnel *net.TCPConn) {
	lock.Lock()
	defer lock.Unlock()

	for key, connMatch := range connectionPool {
		if connMatch.accept != nil {
			go copy2Conn(connMatch.accept, tunnel)
			delete(connectionPool, key)
			return
		}
	}
	_ = tunnel.Close()
}

func cleanConnectionPool() {
	for {
		lock.Lock()
		for key, connMatch := range connectionPool {
			if time.Now().Sub(connMatch.addTime) > time.Second*10 {
				_ = connMatch.accept.Close()
				delete(connectionPool, key)
			}
		}
		lock.Unlock()
		time.Sleep(5 * time.Second)
	}
}
