package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/micr0/zubr-server/internal/logger"
)

// Bot represents an IRC bot that can execute commands
type Bot struct {
	conn           net.Conn
	nick           string
	serverAddr     string
	password       string
	connected      bool
	mu             sync.Mutex
	stopChan       chan bool
	joinedChannels map[string]bool
	channelsMu     sync.RWMutex
}

// NewBot creates a new IRC bot instance
func NewBot(serverAddr, nick, password string) *Bot {
	return &Bot{
		serverAddr:     serverAddr,
		nick:           nick,
		password:       password,
		stopChan:       make(chan bool),
		joinedChannels: make(map[string]bool),
	}
}

// Connect establishes a connection to the IRC server
func (b *Bot) Connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.connected {
		return nil
	}

	logger.Debug("Bot %s connecting to IRC server at %s", b.nick, b.serverAddr)

	conn, err := net.DialTimeout("tcp", b.serverAddr, 5*time.Second)
	if err != nil {
		logger.Error("Bot failed to connect to IRC server: %v", err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	b.conn = conn
	b.connected = true

	logger.Info("Bot %s connected to IRC server", b.nick)

	// Start message handler first
	go b.handleMessages()

	// Send registration commands
	if b.password != "" {
		if err := b.sendRaw(fmt.Sprintf("PASS %s", b.password)); err != nil {
			logger.Error("Bot failed to send PASS: %v", err)
			return err
		}
	}

	if err := b.sendRaw(fmt.Sprintf("NICK %s", b.nick)); err != nil {
		logger.Error("Bot failed to send NICK: %v", err)
		return err
	}

	if err := b.sendRaw(fmt.Sprintf("USER %s 0 * :Zubr Services Bot", b.nick)); err != nil {
		logger.Error("Bot failed to send USER: %v", err)
		return err
	}

	// Authenticate as operator and join default channels after a brief delay
	go func() {
		time.Sleep(2 * time.Second)
		if err := b.sendRaw(fmt.Sprintf("OPER %s %s", b.nick, b.password)); err != nil {
			logger.Error("Bot failed to authenticate as operator: %v", err)
		} else {
			logger.Info("Bot %s authenticated as operator", b.nick)
		}

		// Join default channels after becoming oper
		time.Sleep(1 * time.Second)
		defaultChannels := []string{"#general", "#lobby"}
		for _, channel := range defaultChannels {
			if err := b.JoinChannel(channel); err != nil {
				logger.Error("Bot failed to join %s: %v", channel, err)
			} else {
				b.channelsMu.Lock()
				b.joinedChannels[channel] = true
				b.channelsMu.Unlock()
				logger.Info("Bot joined default channel %s", channel)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	return nil
}

// handleMessages reads and processes messages from the IRC server
func (b *Bot) handleMessages() {
	scanner := bufio.NewScanner(b.conn)
	for {
		select {
		case <-b.stopChan:
			return
		default:
			if scanner.Scan() {
				line := scanner.Text()
				b.processMessage(line)
			} else {
				if err := scanner.Err(); err != nil {
					logger.Error("Bot error reading from IRC: %v", err)
				}
				b.connected = false
				return
			}
		}
	}
}

// processMessage handles incoming IRC messages
func (b *Bot) processMessage(line string) {
	logger.Debug("Bot received: %s", line)

	// Handle PING
	if strings.HasPrefix(line, "PING") {
		pong := strings.Replace(line, "PING", "PONG", 1)
		b.sendRaw(pong)
		return
	}

	// Handle successful JOIN (format: :nick!user@host JOIN :#channel)
	if strings.Contains(line, " JOIN ") && strings.Contains(line, b.nick) {
		parts := strings.Split(line, " ")
		if len(parts) >= 3 {
			channel := strings.TrimPrefix(parts[2], ":")
			channel = strings.TrimSpace(channel)
			if channel != "" {
				b.channelsMu.Lock()
				b.joinedChannels[channel] = true
				b.channelsMu.Unlock()
				logger.Debug("Bot confirmed in channel %s", channel)
			}
		}
	}

	// Log important messages
	if strings.Contains(line, "MODE") || strings.Contains(line, "OPER") {
		logger.Debug("Bot IRC event: %s", line)
	}
}

// sendRaw sends a raw IRC command
func (b *Bot) sendRaw(command string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected || b.conn == nil {
		return fmt.Errorf("bot not connected")
	}

	logger.Debug("Bot sending: %s", command)
	_, err := fmt.Fprintf(b.conn, "%s\r\n", command)
	return err
}

// ensureInChannel makes sure the bot is in a channel before performing operations
func (b *Bot) ensureInChannel(channel string) error {
	b.channelsMu.RLock()
	joined := b.joinedChannels[channel]
	b.channelsMu.RUnlock()

	if joined {
		return nil
	}

	// Join the channel
	logger.Debug("Bot joining channel %s", channel)
	if err := b.sendRaw(fmt.Sprintf("JOIN %s", channel)); err != nil {
		return fmt.Errorf("failed to join channel: %w", err)
	}

	// Mark as joined
	b.channelsMu.Lock()
	b.joinedChannels[channel] = true
	b.channelsMu.Unlock()

	logger.Info("Bot joined channel %s", channel)

	// Wait a moment for the join to complete
	time.Sleep(500 * time.Millisecond)

	return nil
}

// GiveVoice grants voice to a user in a channel
func (b *Bot) GiveVoice(channel, username string) error {
	if err := b.ensureInChannel(channel); err != nil {
		return err
	}
	return b.sendRaw(fmt.Sprintf("MODE %s +v %s", channel, username))
}

// RemoveVoice removes voice from a user in a channel
func (b *Bot) RemoveVoice(channel, username string) error {
	if err := b.ensureInChannel(channel); err != nil {
		return err
	}
	return b.sendRaw(fmt.Sprintf("MODE %s -v %s", channel, username))
}

// SetChannelMode sets a mode on a channel
func (b *Bot) SetChannelMode(channel, mode string) error {
	if err := b.ensureInChannel(channel); err != nil {
		return err
	}
	return b.sendRaw(fmt.Sprintf("MODE %s %s", channel, mode))
}

// KillUser kills/disconnects a user from the server
func (b *Bot) KillUser(username, reason string) error {
	return b.sendRaw(fmt.Sprintf("KILL %s :%s", username, reason))
}

// JoinChannel makes the bot join a channel
func (b *Bot) JoinChannel(channel string) error {
	return b.sendRaw(fmt.Sprintf("JOIN %s", channel))
}

// Disconnect closes the bot connection
func (b *Bot) Disconnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil
	}

	close(b.stopChan)
	b.sendRaw("QUIT :Zubr Services Bot shutting down")

	if b.conn != nil {
		err := b.conn.Close()
		b.conn = nil
		b.connected = false
		logger.Info("Bot %s disconnected", b.nick)
		return err
	}

	return nil
}

// IsConnected returns whether the bot is currently connected
func (b *Bot) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connected
}
