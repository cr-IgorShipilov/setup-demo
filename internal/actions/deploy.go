package actions

import (
	"fmt"
	"time"

	"setup-demo/internal/config"
	"setup-demo/internal/logging"
)

// event mirrors the structured JSON events that, in the real
// implementation, would arrive over a named pipe from the custom Ansible
// callback plugin (see architecture notes). For this demo build there is
// no real Ansible run — eventsFor() below generates a fake but realistic
// event sequence in-process instead of reading a FIFO.
type event struct {
	host   string
	task   string
	status string // "ok" | "changed" | "failed" | "skipped"
}

var demoTasks = []string{
	"Gather facts",
	"Install prerequisites",
	"Configure Indigo service",
	"Deploy application bundle",
	"Start and enable service",
	"Verify health endpoint",
}

// Deploy implements "setup --deploy": it simulates invoking ansible-playbook
// against the configured servers and renders live per-host/per-task status
// as fake events arrive, then writes the full "execution log" to disk.
//
// In production this would launch `ansible-playbook` as a subprocess with
// ANSIBLE_CALLBACK_PLUGINS pointed at the custom plugin, and read structured
// JSON events from a named pipe in real time instead of the in-memory
// channel used here.
func Deploy(serversPath string, log *logging.Logger) error {
	servers, err := config.Load(serversPath)
	if err != nil {
		return fmt.Errorf("load servers.yml: %w", err)
	}
	if len(servers) == 0 {
		servers = config.FakeServers()
		log.Printf("No servers found in %s — running demo deployment against fake hosts.", serversPath)
	}

	log.Printf("Starting deployment to %d server(s)...\n", len(servers))

	events := make(chan event)
	go emitFakeEvents(servers, events)

	failures := 0
	statusByHost := map[string]string{}
	for _, s := range servers {
		statusByHost[s.FQDN] = "pending"
	}

	for ev := range events {
		log.Infof("event host=%s task=%q status=%s", ev.host, ev.task, ev.status)
		symbol := statusSymbol(ev.status)
		fmt.Printf("[%s] %-28s %-24s %s\n", timestamp(), ev.host, ev.task, symbol)

		if ev.status == "failed" {
			failures++
			statusByHost[ev.host] = "failed"
		} else if statusByHost[ev.host] != "failed" {
			statusByHost[ev.host] = "in-progress"
		}
	}

	fmt.Println("\n--- Deployment summary ---")
	for _, s := range servers {
		final := statusByHost[s.FQDN]
		if final != "failed" {
			final = "ok"
		}
		fmt.Printf("  %-28s %s\n", s.FQDN, final)
	}

	if failures > 0 {
		log.Printf("\nDeployment finished with %d failed task(s). See log for details.", failures)
		return fmt.Errorf("%d task(s) failed during deployment", failures)
	}

	log.Printf("\nDeployment finished successfully on all %d server(s).", len(servers))
	return nil
}

// emitFakeEvents produces a plausible stream of per-host/per-task events,
// with a small injected failure so the demo shows both success and
// failure rendering paths, then closes the channel when done.
func emitFakeEvents(servers []config.Server, out chan<- event) {
	defer close(out)

	failHost := ""
	if len(servers) > 0 {
		failHost = servers[len(servers)-1].FQDN // last host "fails" one task, for demo purposes
	}

	for _, s := range servers {
		for i, task := range demoTasks {
			time.Sleep(150 * time.Millisecond) // simulate real execution time

			status := "ok"
			if i == 2 {
				status = "changed"
			}
			if s.FQDN == failHost && task == "Verify health endpoint" {
				status = "failed"
			}
			out <- event{host: s.FQDN, task: task, status: status}
		}
	}
}

func statusSymbol(status string) string {
	switch status {
	case "ok":
		return "OK"
	case "changed":
		return "CHANGED"
	case "failed":
		return "FAILED"
	case "skipped":
		return "SKIPPED"
	default:
		return status
	}
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
