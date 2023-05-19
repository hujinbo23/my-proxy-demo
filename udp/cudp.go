package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	cloudServerAddr := "127.0.0.1:9090"
	localServerAddr := "0.0.0.0:10080"

	cloudConn, err := net.Dial("udp", cloudServerAddr)
	if err != nil {
		fmt.Println("Error connecting to cloud server:", err)
		return
	}
	defer cloudConn.Close()

	localConn, err := net.ListenPacket("udp", localServerAddr)
	if err != nil {
		fmt.Println("Error starting local listener:", err)
		return
	}
	defer localConn.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, _, err := localConn.ReadFrom(buf)
			if err != nil {
				fmt.Println("Error reading from local listener:", err)
				return
			}
			fmt.Printf("Forwarding from local %s to cloud %s\n", localConn.LocalAddr(), cloudConn.RemoteAddr())
			cloudConn.Write(buf[:n])
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := cloudConn.Read(buf)
			if err != nil {
				fmt.Println("Error reading from cloud connection:", err)
				return
			}
			fmt.Printf("Forwarding from cloud %s to local %s\n", cloudConn.RemoteAddr(), localConn.LocalAddr())
			localConn.WriteTo(buf[:n], localConn.LocalAddr())
		}
	}()

	fmt.Println("UDP relay client started. Press Ctrl+C to stop.")
	for {
		time.Sleep(1 * time.Second)
	}
}
