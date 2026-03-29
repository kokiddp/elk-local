package environment

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type hostPortSelection struct {
	label     string
	enabled   bool
	port      *int
	preferred int
	explicit  bool
	preserve  bool
}

type hostPortRange struct {
	start int
	end   int
}

var (
	windowsExcludedTCPPortRangesOnce sync.Once
	windowsExcludedTCPPortRangesData []hostPortRange
	windowsExcludedTCPPortRangesErr  error
)

func assignCreatePorts(manifest *Manifest, options CreateOptions) error {
	return resolveHostPorts([]hostPortSelection{
		{label: "http", enabled: true, port: &manifest.Network.HTTPPort, preferred: DefaultHTTPPort(manifest.Name), explicit: options.HTTPPort > 0},
		{label: "database", enabled: true, port: &manifest.Network.DatabasePort, preferred: DefaultDatabasePort(manifest.Name), explicit: options.DatabasePort > 0},
		{label: "adminer", enabled: manifest.Tooling.Adminer.Enabled, port: &manifest.Tooling.Adminer.Port, preferred: DefaultAdminerPort(manifest.Name), explicit: options.AdminerPort > 0},
		{label: "mailpit http", enabled: manifest.Tooling.Mailpit.Enabled, port: &manifest.Tooling.Mailpit.HTTPPort, preferred: DefaultMailpitHTTPPort(manifest.Name), explicit: options.MailpitHTTPPort > 0},
		{label: "mailpit smtp", enabled: manifest.Tooling.Mailpit.Enabled, port: &manifest.Tooling.Mailpit.SMTPPort, preferred: DefaultMailpitSMTPPort(manifest.Name), explicit: options.MailpitSMTPPort > 0},
		{label: "xdebug client", enabled: manifest.Tooling.Xdebug.Enabled, port: &manifest.Tooling.Xdebug.ClientPort, preferred: DefaultXdebugClientPort(), explicit: options.XdebugPort > 0},
	})
}

func assignUpdatePorts(manifest *Manifest, previous Manifest, options UpdateOptions) error {
	return resolveHostPorts([]hostPortSelection{
		{label: "http", enabled: true, port: &manifest.Network.HTTPPort, preferred: preferredPort(manifest.Network.HTTPPort, DefaultHTTPPort(manifest.Name)), explicit: options.HTTPPort > 0, preserve: options.HTTPPort == 0},
		{label: "database", enabled: true, port: &manifest.Network.DatabasePort, preferred: preferredPort(manifest.Network.DatabasePort, DefaultDatabasePort(manifest.Name)), explicit: options.DatabasePort > 0, preserve: options.DatabasePort == 0},
		{label: "adminer", enabled: manifest.Tooling.Adminer.Enabled, port: &manifest.Tooling.Adminer.Port, preferred: preferredPort(manifest.Tooling.Adminer.Port, DefaultAdminerPort(manifest.Name)), explicit: options.AdminerPort > 0, preserve: manifest.Tooling.Adminer.Enabled && previous.Tooling.Adminer.Enabled && options.AdminerPort == 0},
		{label: "mailpit http", enabled: manifest.Tooling.Mailpit.Enabled, port: &manifest.Tooling.Mailpit.HTTPPort, preferred: preferredPort(manifest.Tooling.Mailpit.HTTPPort, DefaultMailpitHTTPPort(manifest.Name)), explicit: options.MailpitHTTPPort > 0, preserve: manifest.Tooling.Mailpit.Enabled && previous.Tooling.Mailpit.Enabled && options.MailpitHTTPPort == 0},
		{label: "mailpit smtp", enabled: manifest.Tooling.Mailpit.Enabled, port: &manifest.Tooling.Mailpit.SMTPPort, preferred: preferredPort(manifest.Tooling.Mailpit.SMTPPort, DefaultMailpitSMTPPort(manifest.Name)), explicit: options.MailpitSMTPPort > 0, preserve: manifest.Tooling.Mailpit.Enabled && previous.Tooling.Mailpit.Enabled && options.MailpitSMTPPort == 0},
		{label: "xdebug client", enabled: manifest.Tooling.Xdebug.Enabled, port: &manifest.Tooling.Xdebug.ClientPort, preferred: preferredPort(manifest.Tooling.Xdebug.ClientPort, DefaultXdebugClientPort()), explicit: options.XdebugPort > 0, preserve: manifest.Tooling.Xdebug.Enabled && previous.Tooling.Xdebug.Enabled && options.XdebugPort == 0},
	})
}

func resolveHostPorts(selections []hostPortSelection) error {
	used := make(map[int]string)

	for _, selection := range selections {
		if !selection.enabled || !selection.preserve {
			continue
		}

		port := *selection.port
		if !isValidPort(port) {
			return fmt.Errorf("%s port must be between 1 and 65535", selection.label)
		}
		if conflict, exists := used[port]; exists {
			return fmt.Errorf("%s port %d conflicts with %s port", selection.label, port, conflict)
		}

		used[port] = selection.label
	}

	for _, selection := range selections {
		if !selection.enabled || selection.preserve {
			continue
		}

		port := preferredPort(*selection.port, selection.preferred)
		if selection.explicit {
			if !isValidPort(port) {
				return fmt.Errorf("%s port must be between 1 and 65535", selection.label)
			}
			if conflict, exists := used[port]; exists {
				return fmt.Errorf("%s port %d conflicts with %s port", selection.label, port, conflict)
			}
			if !hostTCPPortAvailable(port) {
				return fmt.Errorf("%s port %d is not available", selection.label, port)
			}

			*selection.port = port
			used[port] = selection.label
			continue
		}

		availablePort, err := findAvailableHostTCPPort(port, used)
		if err != nil {
			return fmt.Errorf("assign %s port: %w", selection.label, err)
		}

		*selection.port = availablePort
		used[availablePort] = selection.label
	}

	return nil
}

func preferredPort(current int, fallback int) int {
	if isValidPort(current) {
		return current
	}

	return fallback
}

func findAvailableHostTCPPort(start int, used map[int]string) (int, error) {
	if start < 1024 {
		start = 1024
	}

	for port := start; port <= 65535; port++ {
		if _, exists := used[port]; exists {
			continue
		}
		if hostTCPPortAvailable(port) {
			return port, nil
		}
	}

	for port := 1024; port < start; port++ {
		if _, exists := used[port]; exists {
			continue
		}
		if hostTCPPortAvailable(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no free TCP ports remain")
}

func hostTCPPortAvailable(port int) bool {
	if windowsHostTCPPortExcluded(port) {
		return false
	}

	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}

func windowsHostTCPPortExcluded(port int) bool {
	ranges, err := windowsExcludedTCPPortRanges()
	if err != nil {
		return false
	}

	for _, portRange := range ranges {
		if port >= portRange.start && port <= portRange.end {
			return true
		}
	}

	return false
}

func windowsExcludedTCPPortRanges() ([]hostPortRange, error) {
	if !runningInWSL() {
		return nil, nil
	}

	windowsExcludedTCPPortRangesOnce.Do(func() {
		command := exec.Command("cmd.exe", "/c", "cd /d C:\\ && netsh interface ipv4 show excludedportrange protocol=tcp")
		output, err := command.Output()
		if err != nil {
			windowsExcludedTCPPortRangesErr = err
			return
		}

		windowsExcludedTCPPortRangesData = parseWindowsExcludedTCPPortRanges(string(output))
	})

	return windowsExcludedTCPPortRangesData, windowsExcludedTCPPortRangesErr
}

func parseWindowsExcludedTCPPortRanges(output string) []hostPortRange {
	ranges := make([]hostPortRange, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		start, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		end, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		ranges = append(ranges, hostPortRange{start: start, end: end})
	}

	return ranges
}

func runningInWSL() bool {
	contents, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}

	osRelease := strings.ToLower(string(contents))
	return strings.Contains(osRelease, "microsoft")
}
