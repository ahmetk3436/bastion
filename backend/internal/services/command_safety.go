package services

import (
	"strings"
)

// CommandSafety determines if a command is safe for auto-execution
type CommandSafety struct {
	BaseCommand string
	IsSafe      bool
	Category    string // system, file, network, dangerous
}

// CommandSafetyChecker evaluates command safety
type CommandSafetyChecker struct {
	safeCommands     map[string]bool
	dangerousCommands map[string]bool
}

// NewCommandSafetyChecker creates a new safety checker
func NewCommandSafetyChecker() *CommandSafetyChecker {
	return &CommandSafetyChecker{
		safeCommands: map[string]bool{
			// System info
			"ls": true, "ps": true, "df": true, "free": true, "uptime": true,
			"whoami": true, "pwd": true, "hostname": true, "uname": true,
			"id": true, "groups": true, "date": true, "env": true,
			"top": true, "htop": true, "w": true, "who": true,

			// File reading (safe operations)
			"cat": true, "head": true, "tail": true, "grep": true,
			"find": true, "wc": true, "du": true, "stat": true,
			"file": true, "readlink": true, "basename": true, "dirname": true,

			// Network diagnostics
			"ping": true, "traceroute": true, "nslookup": true, "dig": true,
			"netstat": true, "ss": true, "ip": true, "ifconfig": true,

			// Process management (read-only)
			"pgrep": true, "pidof": true, "lsof": true,

			// Docker (read-only)
			"docker": true, // Will be validated with args

			// Systemctl (read-only)
			"systemctl": true, // Will be validated with args

			// Others
			"which": true, "type": true, "man": true, "history": true,
			"jobs": true, "echo": true, "seq": true, "sleep": true,
			"test": true, "true": true, "false": true, "printf": true,
		},
		dangerousCommands: map[string]bool{
			// File modification
			"rm": true, "mv": true, "cp": true, "touch": true, "mkdir": true,
			"rmdir": true, "chmod": true, "chown": true, "chgrp": true,
			"ln": true, "split": true, "csplit": true,

			// Disk operations
			"dd": true, "mkfs": true, "fdisk": true, "parted": true,
			"mount": true, "umount": true,

			// System control
			"reboot": true, "shutdown": true, "poweroff": true, "halt": true,
			"init": true, "systemctl": true,

			// Package managers
			"apt": true, "apt-get": true, "yum": true, "dnf": true,
			"pacman": true, "pip": true, "npm": true, "yarn": true,

			// Network dangerous
			"curl": true, "wget": true, "nc": true, "netcat": true,
			"ssh": true, "scp": true, "rsync": true,

			// Editors
			"vi": true, "vim": true, "nano": true, "ed": true,

			// Compression
			"tar": true, "zip": true, "unzip": true, "gzip": true,
			"gunzip": true, "xz": true, "unxz": true,

			// Database
			"mysql": true, "psql": true, "mongosh": true, "redis-cli": true,

			// Other dangerous
			"kill": true, "killall": true, "pkill": true, "su": true,
			"sudo": true, "nohup": true, "screen": true, "tmux": true,
			"iptables": true, "ufw": true, "firewall-cmd": true,
		},
	}
}

// ParseCommand extracts the base command from a command string
func (c *CommandSafetyChecker) ParseCommand(input string) (baseCmd string, args []string) {
	cleanCmd := strings.TrimSpace(input)

	// Handle sudo prefix
	cleanCmd = strings.TrimPrefix(cleanCmd, "sudo ")
	cleanCmd = strings.TrimPrefix(cleanCmd, "sudo")

	// Handle pipes - only check the first command
	if idx := strings.Index(cleanCmd, "|"); idx != -1 {
		cleanCmd = strings.TrimSpace(cleanCmd[:idx])
	}

	// Handle redirections - strip them
	if idx := strings.IndexAny(cleanCmd, ">"); idx != -1 {
		cleanCmd = strings.TrimSpace(cleanCmd[:idx])
	}
	if idx := strings.Index(cleanCmd, ">>"); idx != -1 {
		cleanCmd = strings.TrimSpace(cleanCmd[:idx])
	}

	// Handle paths like /bin/ls, ./command
	parts := strings.Fields(cleanCmd)
	if len(parts) == 0 {
		return "", nil
	}

	// Extract base from path
	baseCandidate := parts[0]
	if strings.Contains(baseCandidate, "/") {
		slashParts := strings.Split(baseCandidate, "/")
		baseCmd = slashParts[len(slashParts)-1]
	} else {
		baseCmd = baseCandidate
	}

	args = parts[1:]
	return baseCmd, args
}

// CheckSafety evaluates if a command is safe for auto-execution
func (c *CommandSafetyChecker) CheckSafety(input string) CommandSafety {
	baseCmd, args := c.ParseCommand(input)

	// Check for dangerous flags that make safe commands unsafe
	switch baseCmd {
	case "docker":
		// docker inspect, ps, logs, stats are safe
		// docker rm, stop, kill, exec are not
		if len(args) > 0 {
			dangerousDocker := map[string]bool{
				"rm": true, "rmi": true, "stop": true, "kill": true,
				"exec": true, "run": true, "restart": true,
				"save": true, "load": true, "commit": true,
			}
			if dangerousDocker[args[0]] {
				return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "dangerous"}
			}
		}
		return CommandSafety{BaseCommand: baseCmd, IsSafe: true, Category: "system"}

	case "systemctl":
		// Only status, is-active, is-enabled are safe
		if len(args) > 0 {
			safeSystemctl := map[string]bool{
				"status": true, "is-active": true, "is-enabled": true,
				"is-failed": true, "show": true, "list-units": true,
				"list-timers": true, "list-jobs": true,
			}
			if safeSystemctl[args[0]] {
				return CommandSafety{BaseCommand: baseCmd, IsSafe: true, Category: "system"}
			}
			// Check for dangerous operations
			dangerousSystemctl := map[string]bool{
				"start": true, "stop": true, "restart": true,
				"reload": true, "enable": true, "disable": true,
				"mask": true, "unmask": true, "daemon-reload": true,
			}
			if dangerousSystemctl[args[0]] {
				return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "dangerous"}
			}
		}
		return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "system"}

	case "grep", "find":
		// Check for -delete flag in find
		if baseCmd == "find" {
			for _, arg := range args {
				if arg == "-delete" || arg == "-exec" {
					return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "dangerous"}
				}
			}
		}
		return CommandSafety{BaseCommand: baseCmd, IsSafe: true, Category: "file"}

	case "kill", "killall", "pkill":
		return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "dangerous"}
	}

	// Check whitelist
	if c.safeCommands[baseCmd] {
		category := c.categorizeCommand(baseCmd)
		return CommandSafety{BaseCommand: baseCmd, IsSafe: true, Category: category}
	}

	// Check dangerous list
	if c.dangerousCommands[baseCmd] {
		return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "dangerous"}
	}

	// Unknown commands are unsafe
	return CommandSafety{BaseCommand: baseCmd, IsSafe: false, Category: "unknown"}
}

func (c *CommandSafetyChecker) categorizeCommand(cmd string) string {
	fileCmds := map[string]bool{"ls": true, "cat": true, "head": true, "tail": true,
		"grep": true, "find": true, "wc": true, "du": true, "stat": true, "file": true}
	networkCmds := map[string]bool{"ping": true, "traceroute": true, "nslookup": true,
		"dig": true, "netstat": true, "ss": true, "ip": true, "ifconfig": true}

	if fileCmds[cmd] {
		return "file"
	}
	if networkCmds[cmd] {
		return "network"
	}
	return "system"
}

// Global singleton
var DefaultSafetyChecker = NewCommandSafetyChecker()
