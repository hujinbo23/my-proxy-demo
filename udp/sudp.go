package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	serverAddr := "0.0.0.0:89"
	clientAddr := "0.0.0.0:9090"

	serverConn, err := net.ListenPacket("udp", serverAddr)
	if err != nil {
		fmt.Println("Error starting server listener:", err)
		return
	}
	defer serverConn.Close()

	clientConn, err := net.ListenPacket("udp", clientAddr)
	if err != nil {
		fmt.Println("Error starting client listener:", err)
		return
	}
	defer clientConn.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFrom(buf)
			if err != nil {
				fmt.Println("Error reading from server listener:", err)
				return
			}
			fmt.Printf("Forwarding from server %s to client %s\n", addr, clientConn.LocalAddr())
			clientConn.WriteTo(buf[:n], clientConn.LocalAddr())
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := clientConn.ReadFrom(buf)
			if err != nil {
				fmt.Println("Error reading from client listener:", err)
				return
			}
			fmt.Printf("Forwarding from client %s to server %s\n", addr, serverConn.LocalAddr())
			serverConn.WriteTo(buf[:n], serverConn.LocalAddr())
		}
	}()

	fmt.Println("UDP relay server started. Press Ctrl+C to stop.")
	for {
		time.Sleep(1 * time.Second)
	}
}
