package testenv

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// startupTimeout bounds how long we wait for a launched server to answer PING.
	startupTimeout = 20 * time.Second
	// startAttempts retries a launch, since the ephemeral port we pick can be
	// taken between choosing it and the server binding it.
	startAttempts = 3
)

// instance is a server process this package launched.
type instance struct {
	addr string
	dir  string
	cmd  *exec.Cmd
	log  *syncBuffer
	once sync.Once
}

// Shared returns the address of this engine's process-wide server, launching it
// on first use. Tests share it and isolate themselves by logical database, so
// nothing here may mutate server-global state — use Exclusive for that.
func (e *Engine) Shared(t *testing.T) string {
	t.Helper()
	if e.external != "" {
		return e.external
	}
	e.once.Do(func() {
		e.shared, e.err = startServer(e)
	})
	if e.err != nil {
		t.Fatalf("could not start shared %s server: %v", e.Name, e.err)
	}
	return e.shared.addr
}

// Exclusive launches a server used only by the calling test and stops it during
// cleanup. Use it for anything that touches server-global state:
// notify-keyspace-events, slowlog config, CONFIG SET, FLUSHALL.
//
// extraArgs are appended to the server command line, e.g.
// "--slowlog-log-slower-than", "0".
func (e *Engine) Exclusive(t *testing.T, extraArgs ...string) string {
	t.Helper()
	if e.external != "" {
		t.Skipf("%s is an externally managed server (%s); this test needs an exclusive one",
			e.Name, e.external)
	}
	inst, err := startServer(e, extraArgs...)
	if err != nil {
		t.Fatalf("could not start exclusive %s server: %v", e.Name, err)
	}
	t.Cleanup(inst.stop)
	return inst.addr
}

// acquireDB checks out a logical database on the shared server for the duration
// of the test, so tests running in parallel never see each other's keys.
func (e *Engine) acquireDB(t *testing.T) int {
	t.Helper()
	select {
	case db := <-e.dbs:
		t.Cleanup(func() { e.dbs <- db })
		return db
	case <-time.After(60 * time.Second):
		t.Fatalf("timed out waiting for a free %s test database (all %d in use)", e.Name, dbCount)
		return 0
	}
}

func startServer(e *Engine, extraArgs ...string) (*instance, error) {
	var lastErr error
	for attempt := range startAttempts {
		inst, err := launch(e, extraArgs)
		if err == nil {
			return inst, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return nil, lastErr
}

func launch(e *Engine, extraArgs []string) (*instance, error) {
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("could not reserve a port: %w", err)
	}

	dir, err := os.MkdirTemp("", "kvweb-test-"+e.Name+"-")
	if err != nil {
		return nil, fmt.Errorf("could not create scratch dir: %w", err)
	}

	// Persistence off so tests never touch disk; notifications off so the
	// keyspace-notification tests start from a known state.
	args := []string{
		"--port", strconv.Itoa(port),
		"--bind", "127.0.0.1",
		"--dir", dir,
		"--databases", strconv.Itoa(dbCount),
		"--save", "",
		"--appendonly", "no",
		"--protected-mode", "no",
		"--notify-keyspace-events", "",
		"--daemonize", "no",
		"--loglevel", "warning",
	}
	args = append(args, extraArgs...)

	log := &syncBuffer{}
	cmd := exec.Command(e.binary, args...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Dir = dir

	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("could not start %s: %w", e.binary, err)
	}

	inst := &instance{
		addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		dir:  dir,
		cmd:  cmd,
		log:  log,
	}

	if err := waitReady(inst.addr, startupTimeout); err != nil {
		inst.stop()
		return nil, fmt.Errorf("%s on %s never became ready: %w\nserver log:\n%s",
			e.binary, inst.addr, err, log.String())
	}
	return inst, nil
}

func (i *instance) stop() {
	i.once.Do(func() {
		if i.cmd != nil && i.cmd.Process != nil {
			_ = i.cmd.Process.Kill()
			_ = i.cmd.Wait()
		}
		_ = os.RemoveAll(i.dir)
	})
}

// freePort reserves an ephemeral port by binding and immediately releasing it.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// waitReady polls the address with an inline RESP PING until it answers +PONG.
// Doing this over a raw socket keeps startup independent of the client library
// under test.
func waitReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		err := pingOnce(addr)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out")
	}
	return lastErr
}

func pingOnce(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return err
	}
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(buf[:n]), "+PONG") {
		return fmt.Errorf("unexpected PING reply %q", string(buf[:n]))
	}
	return nil
}

// runVersion executes `<path> --version` with a short timeout.
func runVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", path, err)
	}
	return string(out), nil
}

// readExternalVersion fills in the version of a server this process does not
// manage, by reading the INFO server section over a raw socket.
func (e *Engine) readExternalVersion() error {
	conn, err := net.DialTimeout("tcp", e.external, 5*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("INFO server\r\n")); err != nil {
		return err
	}

	// INFO answers with a single RESP bulk string. Reading its declared length
	// makes this terminate on its own rather than on the socket deadline.
	reader := bufio.NewReader(conn)
	header, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(header, "$") {
		return fmt.Errorf("unexpected INFO reply %q", strings.TrimSpace(header))
	}
	size, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil || size < 0 {
		return fmt.Errorf("unexpected INFO length %q", strings.TrimSpace(header))
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return err
	}
	info := string(payload)

	field := "redis_version:"
	if e.Name == "valkey" && strings.Contains(info, "valkey_version:") {
		field = "valkey_version:"
	}
	for _, line := range strings.Split(info, "\r\n") {
		if !strings.HasPrefix(line, field) {
			continue
		}
		e.Version = strings.TrimPrefix(line, field)
		parts := strings.SplitN(e.Version, ".", 3)
		if len(parts) == 3 {
			e.Major, _ = strconv.Atoi(parts[0])
			e.Minor, _ = strconv.Atoi(parts[1])
			e.Patch, _ = strconv.Atoi(parts[2])
		}
		return nil
	}
	return fmt.Errorf("no %s field in INFO server output", field)
}

// syncBuffer collects a subprocess's output; the process writes from its own
// goroutine while the test may read it for diagnostics.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
