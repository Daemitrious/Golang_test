package natsclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Auth struct {
	Token    string
	Username string
	Password string
}

type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	auth   Auth
}

func Dial(ctx context.Context, rawURL string) (*Client, error) {
	addr, auth, err := parseURL(rawURL)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	client := &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
		auth:   auth,
	}

	if err := client.handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Subscribe(subject string, queue string, sid string) error {
	if subject == "" {
		return errors.New("nats subject is required")
	}
	if sid == "" {
		sid = "1"
	}

	var command string
	if queue == "" {
		command = fmt.Sprintf("SUB %s %s\r\n", subject, sid)
	} else {
		command = fmt.Sprintf("SUB %s %s %s\r\n", subject, queue, sid)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := io.WriteString(c.conn, command)
	return err
}

func (c *Client) Publish(subject string, payload []byte) error {
	if subject == "" {
		return errors.New("nats subject is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := fmt.Fprintf(c.conn, "PUB %s %d\r\n", subject, len(payload))
	if err != nil {
		return err
	}

	if _, err := c.conn.Write(payload); err != nil {
		return err
	}

	_, err = io.WriteString(c.conn, "\r\n")
	return err
}

func (c *Client) PublishJSON(subject string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Publish(subject, payload)
}

func (c *Client) Flush() error {
	c.mu.Lock()
	_, err := io.WriteString(c.conn, "PING\r\n")
	c.mu.Unlock()
	if err != nil {
		return err
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}

		switch {
		case line == "PONG":
			return nil
		case line == "PING":
			if _, err := io.WriteString(c.conn, "PONG\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(line, "-ERR"):
			return errors.New(line)
		case strings.HasPrefix(line, "MSG "):
			if err := c.discardMessage(line); err != nil {
				return err
			}
		}
	}
}

func (c *Client) ReadLoop(ctx context.Context, handler func([]byte)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := c.readLine()
		if err != nil {
			return err
		}

		switch {
		case strings.HasPrefix(line, "MSG "):
			payload, err := c.readMessage(line)
			if err != nil {
				return err
			}
			handler(payload)

		case line == "PING":
			c.mu.Lock()
			_, err := io.WriteString(c.conn, "PONG\r\n")
			c.mu.Unlock()
			if err != nil {
				return err
			}

		case line == "PONG", line == "+OK", strings.HasPrefix(line, "INFO"):
			continue

		case strings.HasPrefix(line, "-ERR"):
			return errors.New(line)
		}
	}
}

func (c *Client) handshake() error {
	line, err := c.readLine()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "INFO") {
		return fmt.Errorf("unexpected nats greeting: %s", line)
	}

	connectPayload := map[string]any{
		"verbose":  false,
		"pedantic": false,
		"lang":     "go-stdlib",
		"version":  "0.1.0",
	}

	if c.auth.Token != "" {
		connectPayload["auth_token"] = c.auth.Token
	}
	if c.auth.Username != "" {
		connectPayload["user"] = c.auth.Username
		connectPayload["pass"] = c.auth.Password
	}
	data, _ := json.Marshal(connectPayload)

	c.mu.Lock()
	_, err = fmt.Fprintf(c.conn, "CONNECT %s\r\nPING\r\n", data)
	c.mu.Unlock()
	if err != nil {
		return err
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}

		switch {
		case line == "PONG":
			return nil
		case line == "PING":
			if _, err := io.WriteString(c.conn, "PONG\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(line, "-ERR"):
			return errors.New(line)
		}
	}
}

func (c *Client) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

func (c *Client) readMessage(header string) ([]byte, error) {
	size, err := messageSize(header)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, err
	}

	crlf := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, crlf); err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) discardMessage(header string) error {
	payload, err := c.readMessage(header)
	if err != nil {
		return err
	}
	_ = payload
	return nil
}

func messageSize(header string) (int, error) {
	parts := strings.Fields(header)
	if len(parts) < 4 {
		return 0, fmt.Errorf("bad nats MSG header: %s", header)
	}

	return strconv.Atoi(parts[len(parts)-1])
}

func parseURL(rawURL string) (string, Auth, error) {
	if rawURL == "" {
		return "localhost:4222", Auth{}, nil
	}

	if !strings.Contains(rawURL, "://") {
		if strings.Contains(rawURL, ":") {
			return rawURL, Auth{}, nil
		}
		return rawURL + ":4222", Auth{}, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", Auth{}, err
	}

	host := parsed.Hostname()
	if host == "" {
		return "", Auth{}, fmt.Errorf("bad nats url: %s", rawURL)
	}

	port := parsed.Port()
	if port == "" {
		port = "4222"
	}

	auth := Auth{}
	if parsed.User != nil {
		username := parsed.User.Username()
		password, hasPassword := parsed.User.Password()
		if hasPassword {
			auth.Username = username
			auth.Password = password
		} else {
			auth.Token = username
		}
	}

	return net.JoinHostPort(host, port), auth, nil
}
