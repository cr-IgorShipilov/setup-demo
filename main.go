// Command setup is a demo build of the internal deployment CLI described
// in "Deployment: prepare CLI tool (binary file) for deployment".
//
// IMPORTANT: this build uses fake/demo data throughout (hardware/software
// facts in --check, and deployment events in --deploy are simulated). It
// exists to demonstrate the CLI's structure, flag handling, file formats,
// and live-status UX — not to run real Ansible. See README.md for what
// would need to change for a production build.
package main

import (
	"flag"
	"fmt"
	"os"

	"setup-demo/internal/actions"
	"setup-demo/internal/logging"
)

const (
	hostsPath   = "hosts.ini"
	serversPath = "servers.yml"
	logDir      = "logs"
)

func main() {
	var (
		doInit      bool
		doConfigure bool
		doCheck     bool
		doDeploy    bool
		doStatus    bool
	)

	flag.BoolVar(&doInit, "init", false, "Generate demo hosts.ini and empty servers.yml")
	flag.BoolVar(&doInit, "i", false, "Shorthand for --init")
	flag.BoolVar(&doConfigure, "configure", false, "Interactively configure 3 servers into servers.yml")
	flag.BoolVar(&doConfigure, "cfg", false, "Shorthand for --configure")
	flag.BoolVar(&doCheck, "check", false, "Verify hardware/software prerequisites against configured servers")
	flag.BoolVar(&doCheck, "c", false, "Shorthand for --check")
	flag.BoolVar(&doDeploy, "deploy", false, "Deploy to the configured environment with live status")
	flag.BoolVar(&doDeploy, "d", false, "Shorthand for --deploy")
	flag.BoolVar(&doStatus, "status", false, "Show a summary of current cluster health")
	flag.BoolVar(&doStatus, "s", false, "Shorthand for --status")

	flag.Usage = printUsage
	flag.Parse()

	selected := countTrue(doInit, doConfigure, doCheck, doDeploy, doStatus)
	if selected != 1 {
		fmt.Fprintln(os.Stderr, "error: exactly one action must be specified")
		printUsage()
		os.Exit(1)
	}

	action, run := selectAction(doInit, doConfigure, doCheck, doDeploy, doStatus)

	log, err := logging.New(logDir, action)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not initialize logging: %v\n", err)
		os.Exit(1)
	}

	runErr := run(log)

	result := "success"
	if runErr != nil {
		result = fmt.Sprintf("failed: %v", runErr)
	}
	log.Close(result)

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		fmt.Fprintf(os.Stderr, "see log for details: %s\n", log.Path())
		os.Exit(1)
	}
}

func countTrue(vals ...bool) int {
	n := 0
	for _, v := range vals {
		if v {
			n++
		}
	}
	return n
}

func selectAction(doInit, doConfigure, doCheck, doDeploy, doStatus bool) (string, func(*logging.Logger) error) {
	switch {
	case doInit:
		return "init", func(log *logging.Logger) error {
			return actions.Init(hostsPath, serversPath, log)
		}
	case doConfigure:
		return "configure", func(log *logging.Logger) error {
			return actions.Configure(hostsPath, serversPath, log)
		}
	case doCheck:
		return "check", func(log *logging.Logger) error {
			return actions.Check(serversPath, log)
		}
	case doStatus:
		return "status", func(log *logging.Logger) error {
			return actions.Status(serversPath, log)
		}
	default: // doDeploy
		return "deploy", func(log *logging.Logger) error {
			return actions.Deploy(serversPath, log)
		}
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `setup - internal deployment CLI (DEMO BUILD, fake data only)

Usage:
  setup --init         (-i)    Generate demo hosts.ini and empty servers.yml
  setup --configure    (-cfg)  Interactively configure 3 servers into servers.yml
  setup --check        (-c)    Verify hardware/software prerequisites
  setup --deploy       (-d)    Deploy to the environment with live status
  setup --status       (-s)    Show a summary of current cluster health

Exactly one action must be specified per invocation.`)
}
