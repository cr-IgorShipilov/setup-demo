package actions

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"setup-demo/internal/config"
	"setup-demo/internal/logging"
)

const serverCount = 3

// Configure implements "setup --configure": it interactively collects
// network settings for exactly 3 servers and writes them to servers.yml,
// overwriting any existing file. Defaults are pre-filled per server from
// hosts.ini (in file order) so an operator can just press Enter to accept
// them instead of retyping the same values.
func Configure(hostsPath, serversPath string, log *logging.Logger) error {
	defaults, err := config.ParseHostsINI(hostsPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", hostsPath, err)
	}
	if len(defaults) == 0 {
		log.Printf("No usable hosts found in %s — no defaults will be suggested.", hostsPath)
	}

	reader := bufio.NewReader(os.Stdin)
	servers := make([]config.Server, 0, serverCount)

	fmt.Printf("Configuring %d servers. Press Enter to accept the bracketed default (from %s), or Ctrl+C to abort.\n\n", serverCount, hostsPath)

	for i := 1; i <= serverCount; i++ {
		fmt.Printf("--- Server %d/%d ---\n", i, serverCount)

		var def config.Server
		if i-1 < len(defaults) {
			def = defaults[i-1]
		}

		s := config.Server{
			FQDN:    promptWithDefault(reader, "  fqdn", def.FQDN),
			IP:      promptWithDefault(reader, "  ip", def.IP),
			Netmask: promptWithDefault(reader, "  netmask", def.Netmask),
			Gateway: promptWithDefault(reader, "  gw", def.Gateway),
			DNS1:    promptWithDefault(reader, "  dns1", def.DNS1),
			DNS2:    promptWithDefault(reader, "  dns2", def.DNS2),
		}
		servers = append(servers, s)
		log.Infof("Collected server %d: %+v", i, s)
	}

	if err := config.Save(serversPath, servers); err != nil {
		return fmt.Errorf("save servers.yml: %w", err)
	}

	log.Printf("Wrote %s with %d servers.", serversPath, len(servers))
	return nil
}

// promptWithDefault asks for a value on the given label, showing def (if
// non-empty) as a bracketed default that a bare Enter will accept. When no
// default is available it falls back to the original behavior: re-prompt
// until a non-blank answer is provided.
func promptWithDefault(reader *bufio.Reader, label, def string) string {
	for {
		if def != "" {
			fmt.Printf("%s [%s]: ", label, def)
		} else {
			fmt.Printf("%s: ", label)
		}
		line, _ := reader.ReadString('\n')
		value := strings.TrimSpace(line)
		if value != "" {
			return value
		}
		if def != "" {
			return def
		}
		fmt.Println("  Value cannot be empty, please try again.")
	}
}
