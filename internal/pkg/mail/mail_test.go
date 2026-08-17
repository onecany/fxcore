package mail

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeSMTP 最小 SMTP 服务器：捕获 MAIL FROM / RCPT TO / DATA 内容，
// 验证 net/smtp 客户端与 Mailer.Send 的完整交互。
type fakeSMTP struct {
	ln   net.Listener
	mu   sync.Mutex
	from string
	to   string
	data string
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSMTP{ln: ln}
	go f.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *fakeSMTP) hostPort() (host, port string) {
	h, p, err := net.SplitHostPort(f.ln.Addr().String())
	if err != nil {
		panic(err)
	}
	return h, p
}

func (f *fakeSMTP) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	rd := bufio.NewReader(conn)
	reply := func(code int, text string) { _, _ = fmt.Fprintf(conn, "%d %s\r\n", code, text) }
	reply(220, "fake smtp ready")

	var from, to string
	var inData bool
	var data strings.Builder
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				inData = false
				reply(250, "OK")
				f.mu.Lock()
				f.from, f.to, f.data = from, to, data.String()
				f.mu.Unlock()
				continue
			}
			data.WriteString(line)
			data.WriteString("\n")
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			// 多行响应：首行 250-，末行 250 + 能力（AUTH 能力在带用户名时必需；
			// 不声明 STARTTLS——SendMail 检测到会升级 TLS，fake 服务器非 TLS）
			_, _ = fmt.Fprintf(conn, "250-fake\r\n250 AUTH PLAIN LOGIN\r\n")
		case strings.HasPrefix(upper, "AUTH"):
			reply(235, "ok")
		case strings.HasPrefix(upper, "MAIL FROM"):
			from = line
			reply(250, "OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			to = line
			reply(250, "OK")
		case strings.HasPrefix(upper, "DATA"):
			inData = true
			reply(354, "go ahead")
		case upper == "QUIT":
			reply(221, "bye")
			return
		default:
			reply(250, "OK")
		}
	}
}

func TestMailerSendNoAuth(t *testing.T) {
	f := startFakeSMTP(t)
	host, port := f.hostPort()
	var p int
	_, _ = fmt.Sscanf(port, "%d", &p)
	m := New(Config{Host: host, Port: p, From: "noreply@fxcore.local"})

	if err := m.Send("user@example.com", "Test Subject", "hello body 你好"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.Contains(f.to, "user@example.com") {
		t.Fatalf("RCPT TO want user@example.com, got %q", f.to)
	}
	if !strings.Contains(f.data, "Subject: Test Subject") {
		t.Fatalf("data want subject, got %q", f.data)
	}
	if !strings.Contains(f.data, "hello body 你好") {
		t.Fatalf("data want body, got %q", f.data)
	}
}

func TestMailerSendWithAuth(t *testing.T) {
	f := startFakeSMTP(t)
	host, port := f.hostPort()
	var p int
	_, _ = fmt.Sscanf(port, "%d", &p)
	m := New(Config{Host: host, Port: p, Username: "ops", Password: "secret", From: "ops@fxcore.local"})

	if err := m.Send("user@example.com", "Subj", "body"); err != nil {
		t.Fatalf("Send with auth: %v", err)
	}
}

func TestMailerDisabled(t *testing.T) {
	m := New(Config{})
	if m.Enabled() {
		t.Fatalf("empty config must be disabled")
	}
	if err := m.Send("a@b.c", "s", "b"); err == nil {
		t.Fatalf("Send on disabled mailer must error")
	}
}

func TestMailerSendRefused(t *testing.T) {
	m := New(Config{Host: "127.0.0.1", Port: 1, From: "noreply@fxcore.local"})
	if err := m.Send("user@example.com", "s", "b"); err == nil {
		t.Fatalf("Send to refused port must error")
	}
}

func TestMailerEmptyFrom(t *testing.T) {
	f := startFakeSMTP(t)
	host, port := f.hostPort()
	var p int
	_, _ = fmt.Sscanf(port, "%d", &p)
	// Username 与 From 都为空 → 无发件人，Send 拒绝
	m := New(Config{Host: host, Port: p})
	if err := m.Send("user@example.com", "s", "b"); err == nil {
		t.Fatalf("Send with empty from must error")
	}
}
