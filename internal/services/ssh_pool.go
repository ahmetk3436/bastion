package services

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	maxConnsPerServer = 5
	idleTimeout       = 10 * time.Minute
	keepAliveInterval = 30 * time.Second
)

type SSHConn struct {
	Client    *ssh.Client
	LastUsed  time.Time
	ServerKey string
}

type SSHPool struct {
	mu    sync.Mutex
	conns map[string][]*SSHConn // key: "host:port"
}

func NewSSHPool() *SSHPool {
	pool := &SSHPool{
		conns: make(map[string][]*SSHConn),
	}
	go pool.cleanupLoop()
	return pool
}

func (p *SSHPool) GetConnection(host string, port int, username, password, privateKey, authType string) (*ssh.Client, error) {
	key := fmt.Sprintf("%s:%d", host, port)

	p.mu.Lock()
	// Try to find an idle connection
	if conns, ok := p.conns[key]; ok {
		for i, conn := range conns {
			if conn.Client != nil {
				// Test if still alive
				_, _, err := conn.Client.SendRequest("keepalive@bastion", true, nil)
				if err == nil {
					conn.LastUsed = time.Now()
					p.mu.Unlock()
					slog.Debug("Reusing SSH connection", "host", key)
					return conn.Client, nil
				}
				// Dead connection, remove
				conn.Client.Close()
				conns[i] = conns[len(conns)-1]
				p.conns[key] = conns[:len(conns)-1]
			}
		}
	}
	p.mu.Unlock()

	// Create new connection
	client, err := p.dial(host, port, username, password, privateKey, authType)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.conns[key] = append(p.conns[key], &SSHConn{
		Client:    client,
		LastUsed:  time.Now(),
		ServerKey: key,
	})
	p.mu.Unlock()

	// Start keepalive
	go p.keepAlive(client, key)

	return client, nil
}

func (p *SSHPool) dial(host string, port int, username, password, privateKey, authType string) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	switch authType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default: // password
		authMethods = append(authMethods, ssh.Password(password))
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	slog.Info("SSH connection established", "host", addr, "user", username)
	return client, nil
}

func (p *SSHPool) keepAlive(client *ssh.Client, key string) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for range ticker.C {
		_, _, err := client.SendRequest("keepalive@bastion", true, nil)
		if err != nil {
			slog.Debug("SSH keepalive failed, connection dead", "host", key)
			return
		}
	}
}

func (p *SSHPool) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		for key, conns := range p.conns {
			alive := conns[:0]
			for _, conn := range conns {
				if time.Since(conn.LastUsed) > idleTimeout {
					slog.Debug("Closing idle SSH connection", "host", key)
					conn.Client.Close()
				} else {
					alive = append(alive, conn)
				}
			}
			if len(alive) == 0 {
				delete(p.conns, key)
			} else {
				p.conns[key] = alive
			}
		}
		p.mu.Unlock()
	}
}

func (p *SSHPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, conns := range p.conns {
		for _, conn := range conns {
			conn.Client.Close()
		}
		delete(p.conns, key)
	}
	slog.Info("All SSH connections closed")
}

// TestConnection tests an SSH connection without pooling
func TestSSHConnection(host string, port int, username, password, privateKey, authType string) (string, error) {
	var authMethods []ssh.AuthMethod

	switch authType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		authMethods = append(authMethods, ssh.Password(password))
	}

	var fingerprint string
	config := &ssh.ClientConfig{
		User: username,
		Auth: authMethods,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fingerprint = ssh.FingerprintSHA256(key)
			return nil
		},
		Timeout: 10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	// Run a simple test command
	session, err := client.NewSession()
	if err != nil {
		return fingerprint, fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()

	_, err = session.Output("echo ok")
	if err != nil {
		return fingerprint, fmt.Errorf("test command failed: %w", err)
	}

	return fingerprint, nil
}
