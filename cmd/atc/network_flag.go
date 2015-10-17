package main

import "fmt"

type NetworkFlag string

func (net *NetworkFlag) UnmarshalFlag(value string) error {
	switch value {
	// stolen from go stdlib; they don't expose these anywhere convenient :(
	case "tcp", "tcp4", "tcp6":
	case "udp", "udp4", "udp6":
	case "ip", "ip4", "ip6":
	case "unix", "unixgram", "unixpacket":
	default:
		return fmt.Errorf("unknown network: '%s'", value)
	}

	*net = NetworkFlag(value)

	return nil
}
