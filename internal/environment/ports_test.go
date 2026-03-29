package environment

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestCreateSkipsUnavailableAutoAssignedPorts(t *testing.T) {
	t.Parallel()

	name := findNameWithAvailableDefaultPorts(t, "auto-ports-create", true, true, false)
	occupyTCPPort(t, DefaultHTTPPort(name))
	occupyTCPPort(t, DefaultDatabasePort(name))
	occupyTCPPort(t, DefaultAdminerPort(name))
	occupyTCPPort(t, DefaultMailpitHTTPPort(name))
	occupyTCPPort(t, DefaultMailpitSMTPPort(name))

	created, err := Create(CreateOptions{
		Name:           name,
		Preset:         "custom",
		ProjectRoot:    t.TempDir(),
		AdminerEnabled: true,
		MailpitEnabled: true,
		Installer:      stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.Manifest.Network.HTTPPort == DefaultHTTPPort(name) {
		t.Fatalf("expected http port to move away from unavailable default %d", created.Manifest.Network.HTTPPort)
	}
	if created.Manifest.Network.DatabasePort == DefaultDatabasePort(name) {
		t.Fatalf("expected database port to move away from unavailable default %d", created.Manifest.Network.DatabasePort)
	}
	if created.Manifest.Tooling.Adminer.Port == DefaultAdminerPort(name) {
		t.Fatalf("expected adminer port to move away from unavailable default %d", created.Manifest.Tooling.Adminer.Port)
	}
	if created.Manifest.Tooling.Mailpit.HTTPPort == DefaultMailpitHTTPPort(name) {
		t.Fatalf("expected mailpit http port to move away from unavailable default %d", created.Manifest.Tooling.Mailpit.HTTPPort)
	}
	if created.Manifest.Tooling.Mailpit.SMTPPort == DefaultMailpitSMTPPort(name) {
		t.Fatalf("expected mailpit smtp port to move away from unavailable default %d", created.Manifest.Tooling.Mailpit.SMTPPort)
	}
}

func TestCreateRejectsUnavailableExplicitMailpitSMTPPort(t *testing.T) {
	t.Parallel()

	_, busyPort := occupyEphemeralTCPPort(t)

	_, err := Create(CreateOptions{
		Name:            "busy-explicit-mailpit",
		Preset:          "custom",
		ProjectRoot:     t.TempDir(),
		MailpitEnabled:  true,
		MailpitSMTPPort: busyPort,
		Installer:       stubApplicationInstaller{},
	})
	if err == nil {
		t.Fatal("expected create to reject an unavailable explicit mailpit smtp port")
	}
	if !strings.Contains(err.Error(), "mailpit smtp port") || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateEnableMailpitSkipsUnavailableStoredPorts(t *testing.T) {
	t.Parallel()

	name := findNameWithAvailableDefaultPorts(t, "auto-ports-update", false, true, false)
	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        name,
		Preset:      "custom",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	occupyTCPPort(t, created.Manifest.Tooling.Mailpit.HTTPPort)
	occupyTCPPort(t, created.Manifest.Tooling.Mailpit.SMTPPort)

	updated, err := Update(UpdateOptions{
		Name:          created.Manifest.Name,
		ProjectRoot:   projectRoot,
		EnableMailpit: true,
	})
	if err != nil {
		t.Fatalf("enable mailpit: %v", err)
	}

	if updated.Manifest.Tooling.Mailpit.HTTPPort == created.Manifest.Tooling.Mailpit.HTTPPort {
		t.Fatalf("expected mailpit http port to move away from unavailable stored port %d", updated.Manifest.Tooling.Mailpit.HTTPPort)
	}
	if updated.Manifest.Tooling.Mailpit.SMTPPort == created.Manifest.Tooling.Mailpit.SMTPPort {
		t.Fatalf("expected mailpit smtp port to move away from unavailable stored port %d", updated.Manifest.Tooling.Mailpit.SMTPPort)
	}
}

func TestParseWindowsExcludedTCPPortRanges(t *testing.T) {
	t.Parallel()

	output := `
Intervallo di esclusione porte protocollo tcp

Porta iniziale    Porta finale
----------    --------
      5357        5357
     60616       60715
     64042       64141

* - Esclusioni porte amministrate.
`

	ranges := parseWindowsExcludedTCPPortRanges(output)
	if len(ranges) != 3 {
		t.Fatalf("expected 3 excluded ranges, got %d", len(ranges))
	}
	if ranges[1].start != 60616 || ranges[1].end != 60715 {
		t.Fatalf("unexpected middle excluded range: %+v", ranges[1])
	}
}

func findNameWithAvailableDefaultPorts(t *testing.T, prefix string, includeAdminer bool, includeMailpit bool, includeXdebug bool) string {
	t.Helper()

	for index := 0; index < 500; index++ {
		name := fmt.Sprintf("%s-%d", prefix, index)
		ports := []int{DefaultHTTPPort(name), DefaultDatabasePort(name)}
		if includeAdminer {
			ports = append(ports, DefaultAdminerPort(name))
		}
		if includeMailpit {
			ports = append(ports, DefaultMailpitHTTPPort(name), DefaultMailpitSMTPPort(name))
		}
		if includeXdebug {
			ports = append(ports, DefaultXdebugClientPort())
		}

		if allPortsAvailable(ports) {
			return name
		}
	}

	t.Fatal("unable to find a free default port set for test")
	return ""
}

func allPortsAvailable(ports []int) bool {
	seen := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if _, exists := seen[port]; exists {
			return false
		}
		seen[port] = struct{}{}
		if !hostTCPPortAvailable(port) {
			return false
		}
	}

	return true
}

func occupyTCPPort(t *testing.T, port int) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("listen on port %d: %v", port, err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	return listener
}

func occupyEphemeralTCPPort(t *testing.T) (net.Listener, int) {
	t.Helper()

	listener, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address type %T", listener.Addr())
	}

	return listener, address.Port
}
