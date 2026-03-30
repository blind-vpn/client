package client

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

type ServerLatency struct {
	Server  ServerInfo
	Latency time.Duration
	Error   error
}

func PingServers(servers []ServerInfo) []ServerLatency {
	results := make([]ServerLatency, len(servers))
	var wg sync.WaitGroup

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, s ServerInfo) {
			defer wg.Done()
			latency, err := tcpPing(s.PublicIP, s.WGPort)
			results[idx] = ServerLatency{
				Server:  s,
				Latency: latency,
				Error:   err,
			}
		}(i, srv)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Error != nil {
			return false
		}
		if results[j].Error != nil {
			return true
		}
		return results[i].Latency < results[j].Latency
	})

	return results
}

func tcpPing(host string, port int) (time.Duration, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}
