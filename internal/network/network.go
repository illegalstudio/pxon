package network

import (
	"fmt"
	"net"
	"strings"

	"pxon/internal/config"
)

func BuildNet0(cfg config.NetworkConfig, usedIPs map[string]struct{}) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	bridge := strings.TrimSpace(cfg.Bridge)
	if bridge == "" {
		return "", fmt.Errorf("missing network bridge")
	}

	switch mode {
	case "dhcp":
		return fmt.Sprintf("name=eth0,bridge=%s,ip=dhcp", bridge), nil
	case "pool":
		ip, err := NextAvailableIP(cfg.RangeStart, cfg.RangeEnd, usedIPs)
		if err != nil {
			return "", err
		}

		prefix, err := PrefixFromConfig(cfg)
		if err != nil {
			return "", err
		}

		gateway := strings.TrimSpace(cfg.Gateway)
		if gateway == "" {
			return "", fmt.Errorf("missing network gateway for pool mode")
		}

		return fmt.Sprintf("name=eth0,bridge=%s,ip=%s/%d,gw=%s", bridge, ip, prefix, gateway), nil
	default:
		return "", fmt.Errorf("unsupported network mode %q; use dhcp or pool", cfg.Mode)
	}
}

func NextAvailableIP(start, end string, usedIPs map[string]struct{}) (string, error) {
	startIP := net.ParseIP(strings.TrimSpace(start)).To4()
	if startIP == nil {
		return "", fmt.Errorf("invalid range start IP %q", start)
	}

	endIP := net.ParseIP(strings.TrimSpace(end)).To4()
	if endIP == nil {
		return "", fmt.Errorf("invalid range end IP %q", end)
	}

	current := ipToUint32(startIP)
	last := ipToUint32(endIP)
	if current > last {
		return "", fmt.Errorf("invalid IP range: start %s is after end %s", start, end)
	}

	for ; current <= last; current++ {
		ip := uint32ToIP(current).String()
		if _, exists := usedIPs[ip]; exists {
			continue
		}

		return ip, nil
	}

	return "", fmt.Errorf("no free IP available in range %s-%s", start, end)
}

func ipToUint32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(value uint32) net.IP {
	return net.IPv4(
		byte(value>>24),
		byte(value>>16),
		byte(value>>8),
		byte(value),
	)
}

func PrefixFromConfig(cfg config.NetworkConfig) (int, error) {
	if strings.TrimSpace(cfg.Netmask) != "" {
		return PrefixFromNetmask(cfg.Netmask)
	}

	if cfg.CIDR > 0 && cfg.CIDR <= 32 {
		return cfg.CIDR, nil
	}

	return 0, fmt.Errorf("missing network mask")
}

func PrefixFromNetmask(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("missing network mask")
	}

	if strings.Contains(value, ".") {
		ip := net.ParseIP(value).To4()
		if ip == nil {
			return 0, fmt.Errorf("invalid network mask %q", value)
		}

		ones, bits := net.IPMask(ip).Size()
		if bits != 32 || ones == 0 {
			return 0, fmt.Errorf("invalid network mask %q", value)
		}

		return ones, nil
	}

	if strings.HasPrefix(value, "/") {
		value = strings.TrimPrefix(value, "/")
	}

	ipNet := "0.0.0.0/" + value
	_, network, err := net.ParseCIDR(ipNet)
	if err != nil {
		return 0, fmt.Errorf("invalid network mask %q", value)
	}

	ones, _ := network.Mask.Size()
	return ones, nil
}
