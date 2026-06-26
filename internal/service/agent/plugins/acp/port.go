package acp

import (
	"net"
	"strconv"
)

// FindFreePort finds an available TCP port on localhost.
func FindFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// ExtractPortFromArgs extracts the --port value from CLI args.
// Returns 0 if no --port flag is found.
func ExtractPortFromArgs(args []string) int {
	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				return p
			}
		}
	}
	return 0
}
