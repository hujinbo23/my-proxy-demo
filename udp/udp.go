package main

import (
	"encoding/base64"
	"net"
	"sync"
	"time"
)

func main() {

}

type UDPPacket struct {
	Content    string
	LocalAddr  *net.UDPAddr
	RemoteAddr *net.UDPAddr
}

type Message struct {
	Body []byte
	Addr string
}

func NewUDPPacket(buf []byte, laddr, raddr *net.UDPAddr) *UDPPacket {
	return &UDPPacket{
		Content:    base64.StdEncoding.EncodeToString(buf),
		LocalAddr:  laddr,
		RemoteAddr: raddr,
	}
}

func GetContent(m *UDPPacket) (buf []byte, err error) {
	buf, err = base64.StdEncoding.DecodeString(m.Content)
	return
}

func ForwardUserConn(udpConn *net.UDPConn, readCh <-chan *UDPPacket, sendCh chan<- *UDPPacket, bufSize int) {

	//read
	go func() {
		for {
			select {
			case udpMsg, ok := <-readCh:
				if !ok {
					return
				}
				buf, err := GetContent(udpMsg)
				if err != nil {
					continue
				}
				_, _ = udpConn.WriteToUDP(buf, udpMsg.RemoteAddr)
			}
		}
	}()

	// write
	buf := make([]byte, bufSize)
	for {
		n, remoteAddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		udpMsg := NewUDPPacket(buf[:n], nil, remoteAddr)
		select {
		case sendCh <- udpMsg:
		default:
		}
	}

}

func Forwarder(dstAddr *net.UDPAddr, readCh <-chan *UDPPacket, sendCh chan<- *UDPPacket, bufSize int) {
	var mu sync.RWMutex
	udpConnMap := make(map[string]*net.UDPConn)

	// read from dstAddr and write to sendCh
	writerFn := func(raddr *net.UDPAddr, udpConn *net.UDPConn) {
		addr := raddr.String()
		defer func() {
			mu.Lock()
			delete(udpConnMap, addr)
			mu.Unlock()
			udpConn.Close()
		}()

		buf := make([]byte, bufSize)
		for {
			_ = udpConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, _, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			udpMsg := NewUDPPacket(buf[:n], nil, raddr)
			select {
			case sendCh <- udpMsg:
			default:
			}
		}
	}

	// read from readCh
	go func() {
		for udpMsg := range readCh {
			buf, err := GetContent(udpMsg)
			if err != nil {
				continue
			}
			mu.Lock()
			udpConn, ok := udpConnMap[udpMsg.RemoteAddr.String()]
			if !ok {
				udpConn, err = net.DialUDP("udp", nil, dstAddr)
				if err != nil {
					mu.Unlock()
					continue
				}
				udpConnMap[udpMsg.RemoteAddr.String()] = udpConn
			}
			mu.Unlock()

			_, err = udpConn.Write(buf)
			if err != nil {
				udpConn.Close()
			}

			if !ok {
				go writerFn(udpMsg.RemoteAddr, udpConn)
			}
		}
	}()
}


