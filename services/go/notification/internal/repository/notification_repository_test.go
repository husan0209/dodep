package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

// ─── Fake dbConn (scripted pgx responses, no live database) ─────────────────

type fakeRow struct {
	vals []any
	err  error
}

func assignValue(dst any, v any) error {
	switch d := dst.(type) {
	case *string:
		switch t := v.(type) {
		case string:
			*d = t
			return nil
		case []byte:
			*d = string(t)
			return nil
		}
	case *int64:
		switch t := v.(type) {
		case int64:
			*d = t
			return nil
		case int32:
			*d = int64(t)
			return nil
		}
	case *int32:
		switch t := v.(type) {
		case int32:
			*d = t
			return nil
		case int64:
			*d = int32(t) // #nosec G115 -- test values are small
			return nil
		}
	case *bool:
		if t, ok := v.(bool); ok {
			*d = t
			return nil
		}
	case *time.Time:
		if t, ok := v.(time.Time); ok {
			*d = t
			return nil
		}
	case **time.Time:
		if v == nil {
			*d = nil
			return nil
		}
		if t, ok := v.(*time.Time); ok {
			*d = t
			return nil
		}
	case *[]byte:
		if v == nil {
			*d = nil
			return nil
		}
		switch t := v.(type) {
		case []byte:
			*d = t
			return nil
		case string:
			*d = []byte(t)
			return nil
		}
	}
	return fmt.Errorf("assign %T from %T", dst, v)
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("fakeRow: %d dest for %d values", len(dest), len(r.vals))
	}
	for i := range dest {
		if err := assignValue(dest[i], r.vals[i]); err != nil {
			return err
		}
	}
	return nil
}

type fakeRows struct {
	rows [][]any
	pos  int
	err  error
}

func (r *fakeRows) Close()                                         {}
func (r *fakeRows) Err() error                                     { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                  { return pgconn.NewCommandTag("SELECT 1") }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *fakeRows) Next() bool {
	if r.pos < len(r.rows) {
		r.pos++
		return true
	}
	return false
}
func (r *fakeRows) Scan(dest ...any) error {
	return (&fakeRow{vals: r.rows[r.pos-1]}).Scan(dest...)
}
func (r *fakeRows) Values() ([]any, error) { return r.rows[r.pos-1], nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

type recordedCall struct {
	sql  string
	args []any
}

type fakeDB struct {
	mu       sync.Mutex
	calls    []recordedCall
	onQueryRow func(sql string, args []any) pgx.Row
	onQuery    func(sql string, args []any) (pgx.Rows, error)
	onExec     func(sql string, args []any) (pgconn.CommandTag, error)
}

func (f *fakeDB) record(sql string, args []any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedCall{sql: sql, args: args})
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.record(sql, args)
	return f.onExec(sql, args)
}

func (f *fakeDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.record(sql, args)
	return f.onQuery(sql, args)
}

func (f *fakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.record(sql, args)
	return f.onQueryRow(sql, args)
}

func okTag(cmd string) pgconn.CommandTag { return pgconn.NewCommandTag(cmd) }

func newFakeDB() *fakeDB {
	return &fakeDB{
		onQueryRow: func(string, []any) pgx.Row { return &fakeRow{err: pgx.ErrNoRows} },
		onQuery:    func(string, []any) (pgx.Rows, error) { return &fakeRows{}, nil },
		onExec:     func(string, []any) (pgconn.CommandTag, error) { return okTag("SELECT 1"), nil },
	}
}

func withFakeDB(fn func(db *fakeDB)) *NotificationRepository {
	db := newFakeDB()
	fn(db)
	return &NotificationRepository{db: db}
}

// notifRow builds the 16 canonical notification columns.
func notifRow(id string, userID int64) []any {
	now := time.Now()
	return []any{
		id, userID, "in_app", "bet_settled", "normal",
		"Subject", "Message", []byte(`{"bet_id":"b1"}`),
		false, now, (*time.Time)(nil), (*time.Time)(nil),
		"sent", "", "ref-1", []byte(`{"k":"v"}`),
	}
}

func settingsRow(userID int64) []any {
	return []any{
		userID, true, false, true, true,
		[]byte(`{"bonus_activated":{"email_enabled":true,"sms_enabled":false,"push_enabled":true,"in_app_enabled":true}}`),
		time.Now(),
	}
}

// ─── Minimal RESP2 fake Redis (HELLO/CLIENT/SET/GET/DEL/INCR/DECR) ──────────

type fakeRedisServer struct {
	mu  sync.Mutex
	kv  map[string]string
	exp map[string]time.Time
	ln  net.Listener
}

func startFakeRedis(t *testing.T) (*fakeRedisServer, *redis.Client) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeRedisServer{kv: map[string]string{}, exp: map[string]time.Time{}, ln: ln}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	client := redis.NewClient(&redis.Options{Addr: ln.Addr().String(), Protocol: 2})
	t.Cleanup(func() { _ = client.Close() })
	return s, client
}

func (s *fakeRedisServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeRedisServer) readLine(conn net.Conn, buf []byte) (string, []byte, error) {
	for {
		for i := 0; i+1 < len(buf); i++ {
			if buf[i] == '\r' && buf[i+1] == '\n' {
				return string(buf[:i]), buf[i+2:], nil
			}
		}
		tmp := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(tmp)
		if err != nil {
			return "", buf, err
		}
		buf = append(buf, tmp[:n]...)
	}
}

func (s *fakeRedisServer) readCommand(conn net.Conn, buf []byte) ([]string, []byte, error) {
	line, rest, err := s.readLine(conn, buf)
	if err != nil {
		return nil, rest, err
	}
	buf = rest
	if len(line) == 0 || line[0] != '*' {
		return nil, buf, fmt.Errorf("expected array, got %q", line)
	}
	var argc int
	if _, err := fmt.Sscanf(line[1:], "%d", &argc); err != nil {
		return nil, buf, err
	}
	args := make([]string, 0, argc)
	for i := 0; i < argc; i++ {
		var hdr string
		hdr, buf, err = s.readLine(conn, buf)
		if err != nil {
			return nil, buf, err
		}
		if len(hdr) == 0 || hdr[0] != '$' {
			return nil, buf, fmt.Errorf("expected bulk, got %q", hdr)
		}
		var ln int
		if _, err := fmt.Sscanf(hdr[1:], "%d", &ln); err != nil {
			return nil, buf, err
		}
		for len(buf) < ln+2 {
			tmp := make([]byte, 4096)
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Read(tmp)
			if err != nil {
				return nil, buf, err
			}
			buf = append(buf, tmp[:n]...)
		}
		args = append(args, string(buf[:ln]))
		buf = buf[ln+2:]
	}
	return args, buf, nil
}

func (s *fakeRedisServer) expired(key string) bool {
	if exp, ok := s.exp[key]; ok && time.Now().After(exp) {
		delete(s.kv, key)
		delete(s.exp, key)
		return true
	}
	return false
}

func (s *fakeRedisServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var buf []byte
	for {
		args, rest, err := s.readCommand(conn, buf)
		buf = rest
		if err != nil {
			return
		}
		if len(args) == 0 {
			return
		}
		s.mu.Lock()
		var resp string
		switch strings.ToUpper(args[0]) {
		case "HELLO":
			resp = "*14\r\n" +
				"$6\r\nserver\r\n$5\r\nredis\r\n" +
				"$7\r\nversion\r\n$5\r\n7.0.0\r\n" +
				"$5\r\nproto\r\n:2\r\n" +
				"$2\r\nid\r\n:1\r\n" +
				"$4\r\nmode\r\n$10\r\nstandalone\r\n" +
				"$4\r\nrole\r\n$6\r\nmaster\r\n" +
				"$7\r\nmodules\r\n*0\r\n"
		case "CLIENT":
			resp = "+OK\r\n"
		case "SET":
			if len(args) < 3 {
				resp = "-ERR wrong args\r\n"
				break
			}
			s.kv[args[1]] = args[2]
			delete(s.exp, args[1])
			for i := 3; i+1 < len(args); i += 2 {
				switch strings.ToUpper(args[i]) {
				case "EX":
					var secs int64
					_, _ = fmt.Sscanf(args[i+1], "%d", &secs)
					s.exp[args[1]] = time.Now().Add(time.Duration(secs) * time.Second)
				case "PX":
					var ms int64
					_, _ = fmt.Sscanf(args[i+1], "%d", &ms)
					s.exp[args[1]] = time.Now().Add(time.Duration(ms) * time.Millisecond)
				}
			}
			resp = "+OK\r\n"
		case "GET":
			v, ok := s.kv[args[1]]
			if !ok || s.expired(args[1]) {
				resp = "$-1\r\n"
			} else {
				resp = fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
			}
		case "DEL":
			n := 0
			for _, k := range args[1:] {
				if _, ok := s.kv[k]; ok {
					delete(s.kv, k)
					delete(s.exp, k)
					n++
				}
			}
			resp = fmt.Sprintf(":%d\r\n", n)
		case "INCR", "DECR":
			v, ok := s.kv[args[1]]
			var cur int64
			if ok && !s.expired(args[1]) {
				if _, err := fmt.Sscanf(v, "%d", &cur); err != nil {
					resp = "-ERR value is not an integer or out of range\r\n"
					break
				}
			}
			if strings.ToUpper(args[0]) == "INCR" {
				cur++
			} else {
				cur--
			}
			s.kv[args[1]] = fmt.Sprintf("%d", cur)
			resp = fmt.Sprintf(":%d\r\n", cur)
		case "PING":
			resp = "+PONG\r\n"
		default:
			resp = "-ERR unknown command\r\n"
		}
		s.mu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte(resp)); err != nil {
			return
		}
	}
}

func (s *fakeRedisServer) plantRaw(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = val
	delete(s.exp, key)
}

// ─── Nil-client guards (fail-fast preserved) ────────────────────────────────

func TestGetUserUnreadKey(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)

	got := repo.GetUserUnreadKey(12345)
	expected := "notification:unread:12345"

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestNotificationCacheKey(t *testing.T) {
	if got := NotificationCacheKey("n-1"); got != "notification:n-1" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestCacheMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	notif := &Notification{
		ID:        "n-1",
		UserID:    1,
		Channel:   "in_app",
		Type:      "bet_settled",
		Subject:   "subject",
		Message:   "message",
		CreatedAt: time.Now(),
	}

	if err := repo.CacheNotification(ctx, notif, time.Minute); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if _, err := repo.GetCachedNotification(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.InvalidateNotificationCache(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}

func TestUnreadCounterMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	if err := repo.IncrementUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.DecrementUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if _, err := repo.GetUnreadCountFromCache(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.SetUnreadCount(ctx, 1, 10); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}

func TestDatabaseMethods_ReturnErrorWhenDatabaseIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	notif := &Notification{
		ID:        "n-1",
		UserID:    1,
		Channel:   "in_app",
		Type:      "bet_settled",
		Subject:   "subject",
		Message:   "message",
		CreatedAt: time.Now(),
	}

	if err := repo.CreateNotification(ctx, notif); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetNotification(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, _, err := repo.GetUserNotifications(ctx, 1, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.MarkAsRead(ctx, "n-1", 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.MarkAllAsRead(ctx, 1, nil); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.DeleteNotification(ctx, "n-1", 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.UpdateNotificationStatus(ctx, "n-1", "sent", ""); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetNotificationSettings(ctx, 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	settings := &NotificationSettings{
		UserID:          1,
		UpdatedAt:       time.Now(),
		TypePreferences: map[string]ChannelPreferences{},
	}
	if err := repo.UpdateNotificationSettings(ctx, settings); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetPendingNotifications(ctx, 10); err == nil {
		t.Fatalf("expected error when database is nil")
	}
}

// ─── Pure helpers ───────────────────────────────────────────────────────────

func TestNormalizePage(t *testing.T) {
	cases := []struct {
		name       string
		limit      int32
		offset     int32
		wantLimit  int32
		wantOffset int32
	}{
		{"normal", 10, 5, 10, 5},
		{"zero defaults", 0, 0, 20, 0},
		{"negative defaults", -1, 0, 20, 0},
		{"over max clamps", 1000, 0, 100, 0},
		{"negative offset clamps", 10, -4, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, o := normalizePage(tc.limit, tc.offset)
			if l != tc.wantLimit || o != tc.wantOffset {
				t.Fatalf("got (%d,%d), want (%d,%d)", l, o, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestDefaultSettings(t *testing.T) {
	s := defaultSettings(42)
	if s.UserID != 42 || !s.EmailEnabled || s.SMSEnabled || !s.PushEnabled || !s.InAppEnabled {
		t.Fatalf("bad channel defaults: %+v", s)
	}
	if s.TypePreferences == nil {
		t.Fatalf("type preferences must be initialized")
	}
}

func TestMustJSON(t *testing.T) {
	if string(mustJSON(nil)) != "{}" {
		t.Fatalf("nil map must encode as {}")
	}
	if string(mustJSON(map[string]string{"a": "b"})) != `{"a":"b"}` {
		t.Fatalf("map must round-trip")
	}
}

// ─── SQL paths with scripted fakes ──────────────────────────────────────────

func TestCreateNotification(t *testing.T) {
	ctx := context.Background()
	db := newFakeDB()
	var gotArgs []any
	db.onExec = func(_ string, args []any) (pgconn.CommandTag, error) {
		gotArgs = append([]any{}, args...)
		return okTag("INSERT 1"), nil
	}
	repo := &NotificationRepository{db: db}

	n := &Notification{ID: "n-1", UserID: 7, Channel: "in_app", Type: "bet_settled",
		Subject: "s", Message: "m"}
	if err := repo.CreateNotification(ctx, n); err != nil {
		t.Fatalf("CreateNotification: %v", err)
	}
	if n.Status != "pending" || n.Priority != "normal" || n.CreatedAt.IsZero() {
		t.Fatalf("defaults must be applied: %+v", n)
	}
	if len(gotArgs) != 15 {
		t.Fatalf("expected 15 args, got %d", len(gotArgs))
	}
	if gotArgs[1] != int64(7) {
		t.Fatalf("user_id must be int64: %v", gotArgs[1])
	}
	if string(gotArgs[7].([]byte)) != "{}" || string(gotArgs[14].([]byte)) != "{}" {
		t.Fatalf("nil maps must encode as {}")
	}

	if err := repo.CreateNotification(ctx, nil); err == nil {
		t.Fatalf("nil notification must fail")
	}
	if err := repo.CreateNotification(ctx, &Notification{}); err == nil {
		t.Fatalf("empty id must fail")
	}

	dbErr := errors.New("db down")
	db.onExec = func(string, []any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, dbErr }
	if err := repo.CreateNotification(ctx, n); err == nil {
		t.Fatalf("exec error must propagate")
	}
}

func TestGetNotification(t *testing.T) {
	ctx := context.Background()

	repo := withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{vals: notifRow("n-1", 7)}
		}
	})
	n, err := repo.GetNotification(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetNotification: %v", err)
	}
	if n == nil || n.ID != "n-1" || n.UserID != 7 || n.Data["bet_id"] != "b1" || n.Metadata["k"] != "v" {
		t.Fatalf("row mismatch: %+v", n)
	}
	if n.ReadAt != nil || n.SentAt != nil {
		t.Fatalf("null timestamps must stay nil")
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		}
	})
	n, err = repo.GetNotification(ctx, "missing")
	if err != nil || n != nil {
		t.Fatalf("missing row must give (nil,nil), got %+v err %v", n, err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{err: errors.New("db down")}
		}
	})
	if _, err := repo.GetNotification(ctx, "n-1"); err == nil {
		t.Fatalf("query error must propagate")
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{vals: []any{"only-one-col"}}
		}
	})
	if _, err := repo.GetNotification(ctx, "n-1"); err == nil {
		t.Fatalf("scan error must propagate")
	}
}

func TestScanNotificationToleratesBadJSON(t *testing.T) {
	vals := notifRow("n-1", 7)
	vals[7] = []byte("{broken")
	vals[15] = []byte("[broken")
	n, err := scanNotification(&fakeRow{vals: vals})
	if err != nil {
		t.Fatalf("bad JSONB must be tolerated: %v", err)
	}
	if len(n.Data) != 0 || len(n.Metadata) != 0 {
		t.Fatalf("bad JSONB must decode to empty maps")
	}
}

func TestGetUserNotifications(t *testing.T) {
	ctx := context.Background()
	typ := "bet_settled"
	isRead := false
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	db := newFakeDB()
	var gotSQL string
	var gotArgs []any
	db.onQueryRow = func(_ string, _ []any) pgx.Row {
		return &fakeRow{vals: []any{int64(2)}}
	}
	db.onQuery = func(sql string, args []any) (pgx.Rows, error) {
		gotSQL, gotArgs = sql, append([]any{}, args...)
		return &fakeRows{rows: [][]any{notifRow("n-1", 7), notifRow("n-2", 7)}}, nil
	}
	repo := &NotificationRepository{db: db}

	out, total, err := repo.GetUserNotifications(ctx, 7, &typ, &isRead, &from, &to, 10, 0)
	if err != nil {
		t.Fatalf("GetUserNotifications: %v", err)
	}
	if total != 2 || len(out) != 2 || out[0].ID != "n-1" || out[1].ID != "n-2" {
		t.Fatalf("list mismatch: total=%d out=%+v", total, out)
	}
	for _, want := range []string{"$1", "$2", "$3", "$4", "$5", "LIMIT $", "OFFSET $"} {
		if !strings.Contains(gotSQL, want) {
			t.Fatalf("SQL must be parameterized, missing %s in %q", want, gotSQL)
		}
	}
	if len(gotArgs) != 7 { // user + 4 filters + limit + offset
		t.Fatalf("expected 7 args, got %d", len(gotArgs))
	}

	// No filters: only user + limit + offset.
	gotArgs = nil
	if _, _, err := repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 0, -1); err != nil {
		t.Fatalf("unfiltered list: %v", err)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("expected 3 args without filters, got %d", len(gotArgs))
	}
	if gotArgs[1] != int32(20) || gotArgs[2] != int32(0) {
		t.Fatalf("pagination must normalize, got %v", gotArgs[1:])
	}

	// Empty result is a non-nil slice.
	db.onQueryRow = func(_ string, _ []any) pgx.Row {
		return &fakeRow{vals: []any{int64(0)}}
	}
	db.onQuery = func(string, []any) (pgx.Rows, error) { return &fakeRows{}, nil }
	out, total, err = repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 10, 0)
	if err != nil || total != 0 || out == nil || len(out) != 0 {
		t.Fatalf("empty list must be non-nil: %+v err %v", out, err)
	}

	// Error paths.
	db.onQueryRow = func(_ string, _ []any) pgx.Row {
		return &fakeRow{err: errors.New("db down")}
	}
	if _, _, err := repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("count error must propagate")
	}
	db.onQueryRow = func(_ string, _ []any) pgx.Row {
		return &fakeRow{vals: []any{int64(1)}}
	}
	db.onQuery = func(string, []any) (pgx.Rows, error) { return nil, errors.New("db down") }
	if _, _, err := repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("query error must propagate")
	}
	db.onQuery = func(string, []any) (pgx.Rows, error) {
		return &fakeRows{rows: [][]any{{"bad"}}}, nil
	}
	if _, _, err := repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("scan error must propagate")
	}
	db.onQuery = func(string, []any) (pgx.Rows, error) {
		return &fakeRows{rows: [][]any{notifRow("n-1", 7)}, err: errors.New("iter")}, nil
	}
	if _, _, err := repo.GetUserNotifications(ctx, 7, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("iteration error must propagate")
	}
}

func TestGetUnreadCount(t *testing.T) {
	ctx := context.Background()
	repo := withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{vals: []any{int64(4)}}
		}
	})
	got, err := repo.GetUnreadCount(ctx, 7)
	if err != nil || got != 4 {
		t.Fatalf("got %d err %v", got, err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{err: errors.New("db down")}
		}
	})
	if _, err := repo.GetUnreadCount(ctx, 7); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestMarkAsRead(t *testing.T) {
	ctx := context.Background()
	db := newFakeDB()
	var gotArgs []any
	db.onExec = func(sql string, args []any) (pgconn.CommandTag, error) {
		gotArgs = append([]any{}, args...)
		if !strings.Contains(sql, "is_read = FALSE") {
			t.Errorf("mark must be conditional on unread, got %q", sql)
		}
		return okTag("UPDATE 1"), nil
	}
	repo := &NotificationRepository{db: db}
	if err := repo.MarkAsRead(ctx, "n-1", 7); err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "n-1" || gotArgs[1] != int64(7) {
		t.Fatalf("bad mark args: %v", gotArgs)
	}

	db.onExec = func(string, []any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errors.New("db down")
	}
	if err := repo.MarkAsRead(ctx, "n-1", 7); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestMarkAllAsRead(t *testing.T) {
	ctx := context.Background()
	db := newFakeDB()
	var gotSQL string
	var gotArgs []any
	db.onExec = func(sql string, args []any) (pgconn.CommandTag, error) {
		gotSQL, gotArgs = sql, append([]any{}, args...)
		return okTag("UPDATE 3"), nil
	}
	repo := &NotificationRepository{db: db}

	typ := "bet_settled"
	n, err := repo.MarkAllAsRead(ctx, 7, &typ)
	if err != nil || n != 3 {
		t.Fatalf("got %d err %v", n, err)
	}
	if !strings.Contains(gotSQL, "type = $2") || len(gotArgs) != 2 {
		t.Fatalf("type filter must be parameterized: %q %v", gotSQL, gotArgs)
	}

	n, err = repo.MarkAllAsRead(ctx, 7, nil)
	if err != nil || n != 3 {
		t.Fatalf("got %d err %v", n, err)
	}
	if strings.Contains(gotSQL, "type =") || len(gotArgs) != 1 {
		t.Fatalf("no filter expected: %q %v", gotSQL, gotArgs)
	}

	db.onExec = func(string, []any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errors.New("db down")
	}
	if _, err := repo.MarkAllAsRead(ctx, 7, nil); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestDeleteNotification(t *testing.T) {
	ctx := context.Background()
	repo := withFakeDB(func(db *fakeDB) {
		db.onExec = func(_ string, args []any) (pgconn.CommandTag, error) {
			if len(args) != 2 || args[0] != "n-1" || args[1] != int64(7) {
				return pgconn.CommandTag{}, errors.New("bad args")
			}
			return okTag("DELETE 1"), nil
		}
	})
	if err := repo.DeleteNotification(ctx, "n-1", 7); err != nil {
		t.Fatalf("DeleteNotification: %v", err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onExec = func(string, []any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		}
	})
	if err := repo.DeleteNotification(ctx, "n-1", 7); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestUpdateNotificationStatus(t *testing.T) {
	ctx := context.Background()
	repo := withFakeDB(func(db *fakeDB) {
		db.onExec = func(_ string, args []any) (pgconn.CommandTag, error) {
			if len(args) != 3 || args[1] != "sent" {
				return pgconn.CommandTag{}, errors.New("bad args")
			}
			return okTag("UPDATE 1"), nil
		}
	})
	if err := repo.UpdateNotificationStatus(ctx, "n-1", "sent", ""); err != nil {
		t.Fatalf("UpdateNotificationStatus: %v", err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onExec = func(string, []any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		}
	})
	if err := repo.UpdateNotificationStatus(ctx, "n-1", "sent", ""); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestGetNotificationSettings(t *testing.T) {
	ctx := context.Background()

	repo := withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{vals: settingsRow(7)}
		}
	})
	s, err := repo.GetNotificationSettings(ctx, 7)
	if err != nil {
		t.Fatalf("GetNotificationSettings: %v", err)
	}
	if s.UserID != 7 || !s.EmailEnabled || s.SMSEnabled {
		t.Fatalf("settings mismatch: %+v", s)
	}
	if !s.TypePreferences["bonus_activated"].PushEnabled {
		t.Fatalf("type preferences must decode: %+v", s.TypePreferences)
	}

	// No stored row -> channel defaults, no error.
	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		}
	})
	s, err = repo.GetNotificationSettings(ctx, 7)
	if err != nil || s == nil || s.UserID != 7 || !s.EmailEnabled || s.SMSEnabled {
		t.Fatalf("defaults expected, got %+v err %v", s, err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQueryRow = func(_ string, _ []any) pgx.Row {
			return &fakeRow{err: errors.New("db down")}
		}
	})
	if _, err := repo.GetNotificationSettings(ctx, 7); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestUpdateNotificationSettings(t *testing.T) {
	ctx := context.Background()
	db := newFakeDB()
	var gotSQL string
	var gotArgs []any
	db.onExec = func(sql string, args []any) (pgconn.CommandTag, error) {
		gotSQL, gotArgs = sql, append([]any{}, args...)
		return okTag("INSERT 1"), nil
	}
	repo := &NotificationRepository{db: db}

	s := &NotificationSettings{UserID: 7, EmailEnabled: true,
		TypePreferences: map[string]ChannelPreferences{
			"bet_settled": {EmailEnabled: true},
		}}
	if err := repo.UpdateNotificationSettings(ctx, s); err != nil {
		t.Fatalf("UpdateNotificationSettings: %v", err)
	}
	if !strings.Contains(gotSQL, "ON CONFLICT (user_id)") || len(gotArgs) != 6 {
		t.Fatalf("upsert must target user_id with 6 args: %q %v", gotSQL, gotArgs)
	}

	// Nil preferences initialize to {}.
	s2 := &NotificationSettings{UserID: 8}
	if err := repo.UpdateNotificationSettings(ctx, s2); err != nil {
		t.Fatalf("nil prefs: %v", err)
	}
	if string(gotArgs[5].([]byte)) != "{}" {
		t.Fatalf("nil prefs must encode as {}, got %s", gotArgs[5])
	}

	if err := repo.UpdateNotificationSettings(ctx, nil); err == nil {
		t.Fatalf("nil settings must fail")
	}

	db.onExec = func(string, []any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errors.New("db down")
	}
	if err := repo.UpdateNotificationSettings(ctx, s); err == nil {
		t.Fatalf("error must propagate")
	}
}

func TestGetPendingNotifications(t *testing.T) {
	ctx := context.Background()
	repo := withFakeDB(func(db *fakeDB) {
		db.onQuery = func(_ string, _ []any) (pgx.Rows, error) {
			return &fakeRows{rows: [][]any{notifRow("n-1", 7)}}, nil
		}
	})
	out, err := repo.GetPendingNotifications(ctx, 10)
	if err != nil || len(out) != 1 || out[0].ID != "n-1" {
		t.Fatalf("pending mismatch: %+v err %v", out, err)
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQuery = func(string, []any) (pgx.Rows, error) {
			return nil, errors.New("db down")
		}
	})
	if _, err := repo.GetPendingNotifications(ctx, 10); err == nil {
		t.Fatalf("query error must propagate")
	}

	repo = withFakeDB(func(db *fakeDB) {
		db.onQuery = func(_ string, _ []any) (pgx.Rows, error) {
			return &fakeRows{rows: [][]any{notifRow("n-1", 7)}, err: errors.New("iter")}, nil
		}
	})
	if _, err := repo.GetPendingNotifications(ctx, 10); err == nil {
		t.Fatalf("iteration error must propagate")
	}
}

// ─── Redis paths against fake RESP server ───────────────────────────────────

func sampleNotif() *Notification {
	return &Notification{
		ID: "n-1", UserID: 7, Channel: "in_app", Type: "bet_settled",
		Priority: "normal", Subject: "subject", Message: "message",
		Data: map[string]string{"bet_id": "b1"},
		CreatedAt: time.Now(), Status: "pending",
		Metadata: map[string]string{"k": "v"},
	}
}

func TestNotificationCache_RoundTrip(t *testing.T) {
	_, client := startFakeRedis(t)
	repo := NewNotificationRepository(nil, client)
	ctx := context.Background()

	if err := repo.CacheNotification(ctx, sampleNotif(), time.Minute); err != nil {
		t.Fatalf("CacheNotification: %v", err)
	}
	got, err := repo.GetCachedNotification(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetCachedNotification: %v", err)
	}
	if got == nil || got.ID != "n-1" || got.UserID != 7 || got.Data["bet_id"] != "b1" {
		t.Fatalf("cache round-trip mismatch: %+v", got)
	}
	if err := repo.InvalidateNotificationCache(ctx, "n-1"); err != nil {
		t.Fatalf("InvalidateNotificationCache: %v", err)
	}
	got, err = repo.GetCachedNotification(ctx, "n-1")
	if err != nil || got != nil {
		t.Fatalf("expected miss after invalidate, got %+v err %v", got, err)
	}
}

func TestNotificationCache_MissAndCorrupt(t *testing.T) {
	srv, client := startFakeRedis(t)
	repo := NewNotificationRepository(nil, client)
	ctx := context.Background()

	got, err := repo.GetCachedNotification(ctx, "nope")
	if err != nil || got != nil {
		t.Fatalf("expected (nil,nil) on miss, got %+v err %v", got, err)
	}
	srv.plantRaw(NotificationCacheKey("bad"), "{not-json")
	if _, err := repo.GetCachedNotification(ctx, "bad"); err == nil {
		t.Fatalf("expected unmarshal error on corrupt payload")
	}
}

func TestNotificationCache_ConnectionError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", Protocol: 2})
	defer func() { _ = client.Close() }()
	repo := NewNotificationRepository(nil, client)
	ctx := context.Background()

	if err := repo.CacheNotification(ctx, sampleNotif(), time.Minute); err == nil {
		t.Fatalf("expected error when redis is down")
	}
	if _, err := repo.GetCachedNotification(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when redis is down")
	}
}

func TestUnreadCounters_FakeRedis(t *testing.T) {
	srv, client := startFakeRedis(t)
	repo := NewNotificationRepository(nil, client)
	ctx := context.Background()

	// Missing key reads as 0.
	got, err := repo.GetUnreadCountFromCache(ctx, 7)
	if err != nil || got != 0 {
		t.Fatalf("missing key must read 0, got %d err %v", got, err)
	}

	if err := repo.SetUnreadCount(ctx, 7, 5); err != nil {
		t.Fatalf("SetUnreadCount: %v", err)
	}
	if got, err := repo.GetUnreadCountFromCache(ctx, 7); err != nil || got != 5 {
		t.Fatalf("got %d err %v", got, err)
	}

	// Increment/decrement round-trip (the decrement fix path).
	if err := repo.IncrementUnreadCount(ctx, 7); err != nil {
		t.Fatalf("IncrementUnreadCount: %v", err)
	}
	if got, _ := repo.GetUnreadCountFromCache(ctx, 7); got != 6 {
		t.Fatalf("expected 6 after increment, got %d", got)
	}
	if err := repo.DecrementUnreadCount(ctx, 7); err != nil {
		t.Fatalf("DecrementUnreadCount: %v", err)
	}
	if got, _ := repo.GetUnreadCountFromCache(ctx, 7); got != 5 {
		t.Fatalf("expected 5 after decrement, got %d", got)
	}

	// Corrupt counter value surfaces a scan error.
	srv.plantRaw(repo.GetUserUnreadKey(7), "not-a-number")
	if _, err := repo.GetUnreadCountFromCache(ctx, 7); err == nil {
		t.Fatalf("expected scan error on corrupt counter")
	}
}
