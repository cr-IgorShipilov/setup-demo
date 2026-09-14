package actions

import (
	"fmt"
	"hash/fnv"
	"os"
	"text/tabwriter"

	"setup-demo/internal/config"
	"setup-demo/internal/logging"
)

// requirement describes a single hardware/software check performed against
// each server. In this demo build, results are derived deterministically
// from the server's FQDN (via a hash) rather than a real Ansible fact
// gathering run, so output is stable across repeated demo runs.
type requirement struct {
	name string
	min  string
}

// ANSI color codes for terminal status output. Only PASS is colorized per
// the current requirement; FAIL is left in the terminal's default color.
const (
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

// colorStatus wraps a PASS status in green ANSI escape codes for terminal
// display. It's applied only to the STATUS column (the last tab-separated
// field), so it doesn't disturb tabwriter's column-width calculations for
// the columns before it.
func colorStatus(status string) string {
	if status == "PASS" {
		return ansiGreen + status + ansiReset
	}
	return status
}

var requirements = []requirement{
	{name: "CPU cores", min: ">= 8"},
	{name: "RAM", min: ">= 32GB"},
	{name: "Disk /", min: ">= 200GB free"},
	{name: "Ansible facts", min: "reachable"},
	{name: "Python 3", min: "installed"},
}

// Check implements "setup --check": it reads servers.yml (falling back to
// demo data when missing/empty) and prints a fake hardware/software
// verification table per server.
func Check(serversPath string, log *logging.Logger) error {
	servers, err := config.Load(serversPath)
	if err != nil {
		return fmt.Errorf("load servers.yml: %w", err)
	}

	usingDemoData := len(servers) == 0
	if usingDemoData {
		servers = config.FakeServers()
		log.Printf("No servers found in %s — showing demo data.", serversPath)
	}

	log.Printf("Checking %d server(s)...\n", len(servers))

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tCHECK\tACTUAL\tREQUIRED\tSTATUS")

	allPass := true
	for _, s := range servers {
		facts := fakeFactsFor(s.FQDN)
		for i, req := range requirements {
			actual := facts[i]
			pass := passes(req, actual)
			status := "PASS"
			if !pass {
				status = "FAIL"
				allPass = false
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.FQDN, req.name, actual, req.min, colorStatus(status))
			log.Infof("check host=%s requirement=%q actual=%q status=%s", s.FQDN, req.name, actual, status)
		}
	}
	w.Flush()

	if usingDemoData {
		fmt.Println("\n(demo data — configure real servers with 'setup --configure')")
	}
	if allPass {
		log.Printf("\nAll checks passed.")
	} else {
		log.Printf("\nOne or more checks failed. Review the table above before deploying.")
	}
	return nil
}

// fakeFactsFor deterministically generates plausible-looking hardware/
// software facts for a given host, so the demo is stable and reproducible
// without querying any real infrastructure.
func fakeFactsFor(fqdn string) []string {
	h := fnv.New32a()
	h.Write([]byte(fqdn))
	seed := h.Sum32()

	cpuCores := 4 + int(seed%12) // 4-15 cores
	ramGB := 16 + int((seed/7)%48) // 16-63 GB
	diskGB := 100 + int((seed/13)%400) // 100-499 GB

	pythonOK := seed%5 != 0 // ~80% pass rate, for demo variety
	pythonStatus := "3.11.4"
	if !pythonOK {
		pythonStatus = "not found"
	}

	return []string{
		fmt.Sprintf("%d", cpuCores),
		fmt.Sprintf("%dGB", ramGB),
		fmt.Sprintf("%dGB free", diskGB),
		"reachable",
		pythonStatus,
	}
}

func passes(req requirement, actual string) bool {
	switch req.name {
	case "Python 3":
		return actual != "not found"
	case "CPU cores":
		var cores int
		fmt.Sscanf(actual, "%d", &cores)
		return cores >= 8
	case "RAM":
		var gb int
		fmt.Sscanf(actual, "%dGB", &gb)
		return gb >= 32
	case "Disk /":
		var gb int
		fmt.Sscanf(actual, "%dGB", &gb)
		return gb >= 200
	default:
		return true
	}
}
