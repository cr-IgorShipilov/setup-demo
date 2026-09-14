package actions

import (
	"fmt"
	"hash/fnv"
	"os"
	"text/tabwriter"

	"setup-demo/internal/config"
	"setup-demo/internal/logging"
)

// nodeHealth is one node's row in the cluster status table. In this demo
// build the values are derived from the node's FQDN rather than queried
// from the node itself, so repeated runs render identically.
type nodeHealth struct {
	fqdn    string
	uptime  string
	version string
	status  string
	note    string // populated only for a node that isn't healthy
}

// colorHealth wraps a HEALTHY status in green for terminal display. Like
// colorStatus in check.go it is applied only to the last tab-separated
// column, so the ANSI bytes never reach tabwriter's column-width
// calculation; DEGRADED is left in the terminal's default color.
func colorHealth(status string) string {
	if status == "HEALTHY" {
		return ansiGreen + status + ansiReset
	}
	return status
}

// Status implements "setup --status": it reads servers.yml (falling back to
// demo data when missing/empty) and prints a per-node summary of current
// cluster health — uptime, running version, and health — followed by a
// cluster-level verdict.
//
// Like --check, it always returns nil: --status reports the state of the
// cluster, so a degraded node is something to display, not a reason to
// fail the invocation.
func Status(serversPath string, log *logging.Logger) error {
	servers, err := config.Load(serversPath)
	if err != nil {
		return fmt.Errorf("load servers.yml: %w", err)
	}

	usingDemoData := len(servers) == 0
	if usingDemoData {
		servers = config.FakeServers()
		log.Printf("No servers found in %s — showing demo data.", serversPath)
	}

	log.Printf("Cluster health across %d node(s):\n", len(servers))

	nodes := make([]nodeHealth, 0, len(servers))
	for i, s := range servers {
		nodes = append(nodes, fakeHealthFor(s.FQDN, i == len(servers)-1))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tUPTIME\tVERSION\tSTATUS")

	healthy := 0
	for _, n := range nodes {
		if n.status == "HEALTHY" {
			healthy++
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", n.fqdn, n.uptime, n.version, colorHealth(n.status))
		log.Infof("status node=%s uptime=%s version=%s health=%s note=%q", n.fqdn, n.uptime, n.version, n.status, n.note)
	}
	w.Flush()

	// Reasons go below the table rather than in a column of their own, so
	// the colorized STATUS column stays last (see colorHealth).
	for _, n := range nodes {
		if n.note != "" {
			fmt.Printf("\n  %s: %s\n", n.fqdn, n.note)
		}
	}

	if usingDemoData {
		fmt.Println("\n(demo data — configure real servers with 'setup --configure')")
	}

	verdict := "HEALTHY"
	if healthy != len(nodes) {
		verdict = "DEGRADED"
	}
	log.Printf("\nCluster status: %s (%d/%d nodes healthy).", verdict, healthy, len(nodes))
	return nil
}

// fakeHealthFor deterministically derives a node's uptime, running version
// and health from its FQDN, for the same reason fakeFactsFor in check.go
// hashes rather than randomizes: a live demo should show the same numbers
// every time it is run.
//
// The caller scripts the last node as degraded so a demo exercises both the
// healthy and unhealthy rendering paths, mirroring how emitFakeEvents in
// deploy.go scripts its failure on the last host.
//
// In production these values would come from Ansible fact gathering and the
// service's own health endpoint rather than from a hash.
func fakeHealthFor(fqdn string, scriptDegraded bool) nodeHealth {
	h := fnv.New32a()
	h.Write([]byte(fqdn))
	seed := h.Sum32()

	hours := 24 + int(seed%(90*24)) // 1-90 days of uptime

	n := nodeHealth{
		fqdn:    fqdn,
		uptime:  fmt.Sprintf("%dd %02dh", hours/24, hours%24),
		version: fmt.Sprintf("4.%d.%d", (seed/11)%4+1, (seed/29)%20),
		status:  "HEALTHY",
	}

	if scriptDegraded {
		n.status = "DEGRADED"
		n.note = "indigo service restarted 3 times in the last hour"
	}
	return n
}
