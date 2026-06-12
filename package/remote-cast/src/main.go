package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ssdpAddr        = "239.255.255.250"
	ssdpPort        = 1900
	pendingWindow   = 5 * time.Second
	stateWriteDelay = 250 * time.Millisecond
)

type config struct {
	LANIface      string
	EasyTierIface string
	PhoneSubnets  string
	AllowedTVs    string
	StateFile     string
	PIDFile       string
	Debug         bool
}

type server struct {
	cfg        config
	lanConn    *net.UDPConn
	easyConn   *net.UDPConn
	lanIP      net.IP
	easyIP     net.IP
	allowedTVs []netip.Prefix
	phoneNets  []netip.Prefix

	mu      sync.Mutex
	pending map[string]pendingClient
	devices map[string]device
	writeCh chan struct{}
}

type pendingClient struct {
	Addr     *net.UDPAddr
	Deadline time.Time
}

type device struct {
	IP       string `json:"ip"`
	USN      string `json:"usn,omitempty"`
	ST       string `json:"st,omitempty"`
	Location string `json:"location,omitempty"`
	Server   string `json:"server,omitempty"`
	Warning  string `json:"warning,omitempty"`
	LastSeen string `json:"last_seen"`
}

func main() {
	var cfg config
	flag.StringVar(&cfg.LANIface, "lan-iface", "br-lan", "LAN interface name")
	flag.StringVar(&cfg.EasyTierIface, "easytier-iface", "easytier0", "EasyTier interface name")
	flag.StringVar(&cfg.PhoneSubnets, "phone-subnet", "10.0.0.0/8", "comma or space separated allowed phone CIDRs")
	flag.StringVar(&cfg.AllowedTVs, "allowed-tv-ips", "", "comma or space separated allowed TV IPs/CIDRs")
	flag.StringVar(&cfg.StateFile, "state-file", "/var/run/remote-cast/devices.json", "JSON state file")
	flag.StringVar(&cfg.PIDFile, "pid-file", "/var/run/remote-cast/remote-cast.pid", "PID file")
	flag.BoolVar(&cfg.Debug, "debug", false, "enable debug logging")
	flag.Parse()

	s, err := newServer(cfg)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}
	defer s.close()

	if err := writePID(cfg.PIDFile); err != nil {
		log.Fatalf("write pid file: %v", err)
	}
	defer os.Remove(cfg.PIDFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 4)
	notifySignals(sigCh)

	go func() {
		for sig := range sigCh {
			if isScanSignal(sig) {
				log.Printf("manual SSDP scan requested")
				s.sendActiveSearch()
				continue
			}
			cancel()
			return
		}
	}()

	if err := s.run(ctx); err != nil {
		log.Fatalf("run failed: %v", err)
	}
}

func newServer(cfg config) (*server, error) {
	lanIP, err := firstIPv4(cfg.LANIface)
	if err != nil {
		return nil, fmt.Errorf("LAN interface %q: %w", cfg.LANIface, err)
	}
	easyIP, err := firstIPv4(cfg.EasyTierIface)
	if err != nil {
		return nil, fmt.Errorf("EasyTier interface %q: %w", cfg.EasyTierIface, err)
	}

	allowedTVs, err := parsePrefixes(cfg.AllowedTVs, false)
	if err != nil {
		return nil, fmt.Errorf("allowed TV list: %w", err)
	}
	phoneNets, err := parsePrefixes(cfg.PhoneSubnets, true)
	if err != nil {
		return nil, fmt.Errorf("phone subnet list: %w", err)
	}
	if len(allowedTVs) == 0 {
		log.Printf("warning: allowed_tv_ips is empty; TV responses will be ignored until configured")
	}

	lanConn, err := listenUDP("udp4", &net.UDPAddr{IP: lanIP, Port: 0}, lanIP, false)
	if err != nil {
		return nil, fmt.Errorf("open LAN UDP socket: %w", err)
	}

	easyConn, err := listenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: ssdpPort}, easyIP, true)
	if err != nil {
		lanConn.Close()
		return nil, fmt.Errorf("open EasyTier SSDP socket: %w", err)
	}

	log.Printf("remote-castd started: lan=%s(%s), easytier=%s(%s), phone_subnet=%q",
		cfg.LANIface, lanIP, cfg.EasyTierIface, easyIP, cfg.PhoneSubnets)

	return &server{
		cfg:        cfg,
		lanConn:    lanConn,
		easyConn:   easyConn,
		lanIP:      lanIP,
		easyIP:     easyIP,
		allowedTVs: allowedTVs,
		phoneNets:  phoneNets,
		pending:    make(map[string]pendingClient),
		devices:    make(map[string]device),
		writeCh:    make(chan struct{}, 1),
	}, nil
}

func (s *server) close() {
	if s.lanConn != nil {
		s.lanConn.Close()
	}
	if s.easyConn != nil {
		s.easyConn.Close()
	}
}

func (s *server) run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		s.readEasyTier(ctx)
	}()
	go func() {
		defer wg.Done()
		s.readLAN(ctx)
	}()
	go func() {
		defer wg.Done()
		s.stateWriter(ctx)
	}()

	s.sendActiveSearch()
	<-ctx.Done()
	s.close()
	wg.Wait()
	return nil
}

func (s *server) readEasyTier(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		_ = s.easyConn.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, err := s.easyConn.ReadFromUDP(buf)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.expirePending()
				continue
			}
			log.Printf("read EasyTier UDP failed: %v", err)
			continue
		}
		if !s.allowedPhone(addr.IP) {
			if s.cfg.Debug {
				log.Printf("ignore SSDP from non-phone subnet: %s", addr)
			}
			continue
		}
		packet := append([]byte(nil), buf[:n]...)
		if !isMSearch(packet) {
			continue
		}
		s.addPending(addr)
		if s.cfg.Debug {
			log.Printf("relay M-SEARCH from %s to LAN", addr)
		}
		if _, err := s.lanConn.WriteToUDP(packet, &net.UDPAddr{IP: net.ParseIP(ssdpAddr), Port: ssdpPort}); err != nil {
			log.Printf("send M-SEARCH to LAN failed: %v", err)
		}
	}
}

func (s *server) readLAN(ctx context.Context) {
	buf := make([]byte, 8192)
	for {
		_ = s.lanConn.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, err := s.lanConn.ReadFromUDP(buf)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Printf("read LAN UDP failed: %v", err)
			continue
		}
		if !s.allowedTV(addr.IP) {
			if s.cfg.Debug {
				log.Printf("ignore SSDP response from non-whitelisted TV: %s", addr.IP)
			}
			continue
		}
		packet := append([]byte(nil), buf[:n]...)
		if !isSSDPResponse(packet) {
			continue
		}
		s.rememberDevice(addr.IP, packet)
		s.forwardToPending(packet)
	}
}

func (s *server) sendActiveSearch() {
	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: ssdp:all\r\n\r\n"
	if _, err := s.lanConn.WriteToUDP([]byte(msg), &net.UDPAddr{IP: net.ParseIP(ssdpAddr), Port: ssdpPort}); err != nil {
		log.Printf("active SSDP scan failed: %v", err)
	}
}

func (s *server) addPending(addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *addr
	s.pending[addr.String()] = pendingClient{Addr: &cp, Deadline: time.Now().Add(pendingWindow)}
}

func (s *server) expirePending() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, client := range s.pending {
		if now.After(client.Deadline) {
			delete(s.pending, key)
		}
	}
}

func (s *server) forwardToPending(packet []byte) {
	now := time.Now()
	var clients []*net.UDPAddr
	s.mu.Lock()
	for key, client := range s.pending {
		if now.After(client.Deadline) {
			delete(s.pending, key)
			continue
		}
		clients = append(clients, client.Addr)
	}
	s.mu.Unlock()

	for _, client := range clients {
		if _, err := s.easyConn.WriteToUDP(packet, client); err != nil {
			log.Printf("forward SSDP response to %s failed: %v", client, err)
		}
	}
}

func (s *server) rememberDevice(ip net.IP, packet []byte) {
	headers := parseHeaders(packet)
	warning := locationWarning(headers["LOCATION"])
	dev := device{
		IP:       ip.String(),
		USN:      headers["USN"],
		ST:       headers["ST"],
		Location: headers["LOCATION"],
		Server:   headers["SERVER"],
		Warning:  warning,
		LastSeen: time.Now().Format(time.RFC3339),
	}

	s.mu.Lock()
	s.devices[dev.IP] = dev
	s.mu.Unlock()

	select {
	case s.writeCh <- struct{}{}:
	default:
	}
}

func (s *server) stateWriter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.writeCh:
			timer := time.NewTimer(stateWriteDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if err := s.writeState(); err != nil {
					log.Printf("write state failed: %v", err)
				}
			}
		}
	}
}

func (s *server) writeState() error {
	s.mu.Lock()
	devices := make([]device, 0, len(s.devices))
	for _, dev := range s.devices {
		devices = append(devices, dev)
	}
	s.mu.Unlock()

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})

	if err := os.MkdirAll(pathDir(s.cfg.StateFile), 0755); err != nil {
		return err
	}
	tmp := s.cfg.StateFile + ".tmp"
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfg.StateFile)
}

func (s *server) allowedPhone(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip.To4())
	if !ok {
		return false
	}
	for _, p := range s.phoneNets {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func (s *server) allowedTV(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip.To4())
	if !ok {
		return false
	}
	for _, p := range s.allowedTVs {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func isMSearch(packet []byte) bool {
	first := firstLine(packet)
	return bytes.HasPrefix(bytes.ToUpper(first), []byte("M-SEARCH "))
}

func isSSDPResponse(packet []byte) bool {
	first := bytes.ToUpper(firstLine(packet))
	return bytes.HasPrefix(first, []byte("HTTP/1.1 200")) || bytes.HasPrefix(first, []byte("NOTIFY "))
}

func firstLine(packet []byte) []byte {
	if i := bytes.IndexAny(packet, "\r\n"); i >= 0 {
		return packet[:i]
	}
	return packet
}

func parseHeaders(packet []byte) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(strings.ReplaceAll(string(packet), "\r\n", "\n"), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers[strings.ToUpper(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return headers
}

func locationWarning(raw string) string {
	if raw == "" {
		return "missing LOCATION"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "invalid LOCATION"
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || ip.To4() == nil {
		return "LOCATION is not an IPv4 address"
	}
	return ""
}

func parsePrefixes(raw string, defaultAll bool) ([]netip.Prefix, error) {
	fields := splitList(raw)
	if len(fields) == 0 && defaultAll {
		fields = []string{"0.0.0.0/0"}
	}
	out := make([]netip.Prefix, 0, len(fields))
	for _, field := range fields {
		if !strings.Contains(field, "/") {
			field += "/32"
		}
		p, err := netip.ParsePrefix(field)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", field, err)
		}
		if !p.Addr().Is4() {
			return nil, fmt.Errorf("%q is not IPv4", field)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func splitList(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	parts := strings.Fields(raw)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstIPv4(ifaceName string) (net.IP, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address found")
}

func writePID(path string) error {
	if err := os.MkdirAll(pathDir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

func pathDir(path string) string {
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return "."
}
