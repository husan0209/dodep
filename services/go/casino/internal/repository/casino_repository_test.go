package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─── GORM harnesses (no live database required) ─────────────────────────────
// NOTE: gorm.Open with the postgres driver dials eagerly, so even "dry run"
// handles must be built on an offline *sql.DB. We use a fake database/sql
// driver (offline by construction) wrapped by the postgres dialector:
//   - dryRun=true: SQL is built but never executed (all success paths).
//   - fail mode: every Exec/Query/Begin returns errDbDown (error branches).

var errDbDown = fmt.Errorf("database is down")

type fakeResult struct{ n int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.n, nil }

type fakeRows struct{}

func (r *fakeRows) Columns() []string           { return []string{} }
func (r *fakeRows) Close() error                { return nil }
func (r *fakeRows) Next(_ []driver.Value) error { return io.EOF }

type fakeTx struct{}

func (tx *fakeTx) Commit() error   { return nil }
func (tx *fakeTx) Rollback() error { return nil }

type fakeConn struct{ fail bool }

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, fmt.Errorf("no prepare") }
func (c *fakeConn) Close() error                          { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	if c.fail {
		return nil, errDbDown
	}
	return &fakeTx{}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.fail {
		return nil, errDbDown
	}
	return fakeResult{n: 1}, nil
}

func (c *fakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.fail {
		return nil, errDbDown
	}
	return &fakeRows{}, nil
}

type fakeDriver struct{}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{fail: name == "fail"}, nil
}

var registerFakeOnce sync.Once

func openOfflineDB(t *testing.T, mode string, dryRun bool) *gorm.DB {
	t.Helper()
	registerFakeOnce.Do(func() { sql.Register("casinofake", &fakeDriver{}) })
	sqlDB, err := sql.Open("casinofake", mode)
	if err != nil {
		t.Fatalf("open fake sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(pgdriver.New(pgdriver.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open offline gorm db: %v", err)
	}
	if dryRun {
		db = db.Session(&gorm.Session{DryRun: true})
	}
	return db
}

// openDryRunDB returns a GORM handle that builds SQL without executing it.
// All query-building success paths return nil errors with zero values.
func openDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openOfflineDB(t, "success", true)
}

// openFailingDB returns a GORM handle whose every query fails with errDbDown.
// Covers error branches without any network.
func openFailingDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openOfflineDB(t, "fail", false)
}

// openFakeExecDB returns a GORM handle whose Exec reports RowsAffected=1
// (write success paths) while reads return empty row sets.
func openFakeExecDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openOfflineDB(t, "success", false)
}

// ─── Minimal RESP2 fake Redis (SET/GET/DEL/INCR/DECR) ───────────────────────

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
			// Minimal Redis 7 HELLO 2 reply (array of key/value pairs).
			// go-redis sends this handshake even with Protocol: 2.
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

// plantRaw stores a raw (possibly invalid JSON) value for cache tests.
func (s *fakeRedisServer) plantRaw(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = val
	delete(s.exp, key)
}

// ─── Nil-client guards (fail-fast preserved) ────────────────────────────────

func TestDatabaseMethods_ReturnErrorWhenDatabaseIsNil(t *testing.T) {
	repo := NewCasinoRepository(nil, nil)
	ctx := context.Background()

	session := &GameSession{
		ID:        "s-1",
		UserID:    "u-1",
		GameID:    "g-1",
		StartedAt: time.Now(),
	}

	round := &GameRound{
		ID:        "r-1",
		SessionID: "s-1",
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	if _, _, err := repo.GetGames(ctx, GetGamesOptions{Limit: 10, Offset: 0}); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetGame(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.UpsertGame(ctx, &Game{ID: "g-1", Name: "X"}); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetProviders(ctx, nil); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetProvider(ctx, "p-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.UpsertProvider(ctx, &Provider{ID: "p-1"}); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.CreateGameSession(ctx, session); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, err := repo.GetGameSession(ctx, "s-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.UpdateGameSession(ctx, session); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.EndGameSession(ctx, "s-1", time.Now()); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, _, err := repo.GetSessionsByUser(ctx, "u-1", 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, _, err := repo.GetGameHistory(ctx, "u-1", nil, nil, 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if _, _, err := repo.GetRoundHistory(ctx, "s-1", 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}
	if err := repo.CreateGameRound(ctx, round); err == nil {
		t.Fatalf("expected error when database is nil")
	}
}

func TestCacheMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewCasinoRepository(nil, nil)
	ctx := context.Background()

	game := &Game{
		ID:         "g-1",
		Name:       "Game",
		ReleasedAt: time.Now(),
	}

	if err := repo.CacheGame(ctx, game, time.Minute); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
	if _, err := repo.GetCachedGame(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
	if err := repo.InvalidateGameCache(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}

// ─── Pure helpers ───────────────────────────────────────────────────────────

func TestGameCacheKey(t *testing.T) {
	if got := GameCacheKey("g-1"); got != "casino:game:g-1" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestNormalizePage(t *testing.T) {
	cases := []struct {
		name       string
		limit      int32
		offset     int32
		wantLimit  int
		wantOffset int
	}{
		{"normal", 10, 5, 10, 5},
		{"zero limit defaults", 0, 0, 20, 0},
		{"negative limit defaults", -3, 0, 20, 0},
		{"over max clamps", 1000, 0, 100, 0},
		{"negative offset clamps", 10, -7, 10, 0},
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

func TestNormalizeDecimalAmount(t *testing.T) {
	if got, err := normalizeDecimalAmount(""); err != nil || got != "0" {
		t.Fatalf("empty should normalize to 0, got %q err %v", got, err)
	}
	if got, err := normalizeDecimalAmount("10.50"); err != nil || got == "" {
		t.Fatalf("valid decimal rejected: %q err %v", got, err)
	}
	if _, err := normalizeDecimalAmount("not-a-number"); err == nil {
		t.Fatalf("invalid decimal must fail")
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("Pragmatic Play"); got != "pragmatic-play" {
		t.Fatalf("unexpected slug %q", got)
	}
	if got := slugify("  PG_Soft "); got != "pg-soft" {
		t.Fatalf("unexpected slug %q", got)
	}
}

func TestTableNames(t *testing.T) {
	if (Game{}).TableName() != "games" {
		t.Fatalf("bad games table")
	}
	if (Provider{}).TableName() != "game_providers" {
		t.Fatalf("bad providers table")
	}
	if (GameSession{}).TableName() != "game_sessions" {
		t.Fatalf("bad sessions table")
	}
	if (GameRound{}).TableName() != "game_rounds" {
		t.Fatalf("bad rounds table")
	}
}

// ─── Dry-run success paths (real SQL built, never executed) ─────────────────

func sampleGame() *Game {
	return &Game{
		ID: "g-1", ExternalID: "ext-1", Name: "Sweet Bonanza",
		ProviderID: "pragmatic", ProviderName: "Pragmatic Play",
		Category: "slot", Tags: []string{"sweet", "fruits"},
		SupportedCurrencies: []string{"USD", "EUR"},
		MinBet: "0.10", MaxBet: "100.00",
		Features: GameFeatures{HasFreeSpins: true, BonusFeatures: []string{"multiplier"}},
		RTP: 96.5, Volatility: "high", IsActive: true, IsDemoAvailable: true,
		RestrictedCountries: []string{"US"}, PopularityScore: 99,
		ReleasedAt: time.Now(), Metadata: map[string]string{"k": "v"},
	}
}

func TestGetGames_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	ctx := context.Background()

	prov := "pragmatic"
	cat := "slot"
	search := "sweet"
	games, total, err := repo.GetGames(ctx, GetGamesOptions{
		ProviderID: &prov, Category: &cat,
		Tags:       []string{"sweet"}, Search: &search,
		Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("dry-run GetGames: %v", err)
	}
	if total != 0 || len(games) != 0 {
		t.Fatalf("dry-run must return empty results")
	}

	// Pagination normalization paths.
	if _, _, err := repo.GetGames(ctx, GetGamesOptions{Limit: 0, Offset: -5}); err != nil {
		t.Fatalf("dry-run GetGames defaults: %v", err)
	}
	if _, _, err := repo.GetGames(ctx, GetGamesOptions{Limit: 5000}); err != nil {
		t.Fatalf("dry-run GetGames clamp: %v", err)
	}
}

func TestGetGame_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	game, err := repo.GetGame(context.Background(), "g-1")
	if err != nil {
		t.Fatalf("dry-run GetGame: %v", err)
	}
	if game == nil {
		t.Fatalf("dry-run GetGame must return non-nil")
	}
}

func TestUpsertGame_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	ctx := context.Background()

	if err := repo.UpsertGame(ctx, sampleGame()); err != nil {
		t.Fatalf("dry-run UpsertGame: %v", err)
	}
	// Defaults: external id falls back to id, nil slices/maps initialized.
	g := sampleGame()
	g.ExternalID = ""
	g.Tags = nil
	g.SupportedCurrencies = nil
	g.RestrictedCountries = nil
	g.Metadata = nil
	g.MinBet = ""
	if err := repo.UpsertGame(ctx, g); err != nil {
		t.Fatalf("dry-run UpsertGame defaults: %v", err)
	}
	if g.ExternalID != "g-1" {
		t.Fatalf("external id should default to id")
	}
	if err := repo.UpsertGame(ctx, nil); err == nil {
		t.Fatalf("nil game must fail")
	}
	if err := repo.UpsertGame(ctx, &Game{}); err == nil {
		t.Fatalf("empty id must fail")
	}
	bad := sampleGame()
	bad.MinBet = "NaN-bet"
	if err := repo.UpsertGame(ctx, bad); err == nil {
		t.Fatalf("invalid min_bet must fail")
	}
	bad = sampleGame()
	bad.MaxBet = "NaN-bet"
	if err := repo.UpsertGame(ctx, bad); err == nil {
		t.Fatalf("invalid max_bet must fail")
	}
}

func TestProviders_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	ctx := context.Background()

	active := true
	if _, err := repo.GetProviders(ctx, nil); err != nil {
		t.Fatalf("dry-run GetProviders: %v", err)
	}
	if _, err := repo.GetProviders(ctx, &active); err != nil {
		t.Fatalf("dry-run GetProviders filtered: %v", err)
	}
	if _, err := repo.GetProvider(ctx, "p-1"); err != nil {
		t.Fatalf("dry-run GetProvider: %v", err)
	}

	p := &Provider{ID: "pp", Name: "Pragmatic Play", IsActive: true,
		SupportedCurrencies: []string{"USD"}, Metadata: map[string]string{}}
	if err := repo.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("dry-run UpsertProvider: %v", err)
	}
	if p.Slug != "pragmatic-play" {
		t.Fatalf("slug should derive from name, got %q", p.Slug)
	}
	// Slug falls back to id when name is empty.
	p2 := &Provider{ID: "MyProv"}
	if err := repo.UpsertProvider(ctx, p2); err != nil {
		t.Fatalf("dry-run UpsertProvider id slug: %v", err)
	}
	if p2.Slug != "myprov" {
		t.Fatalf("slug should derive from id, got %q", p2.Slug)
	}
	if err := repo.UpsertProvider(ctx, nil); err == nil {
		t.Fatalf("nil provider must fail")
	}
	if err := repo.UpsertProvider(ctx, &Provider{}); err == nil {
		t.Fatalf("empty provider id must fail")
	}
}

func sampleSession() *GameSession {
	now := time.Now()
	return &GameSession{
		ID: "s-1", UserID: "u-1", GameID: "g-1", ProviderID: "pragmatic",
		Status: "active", BalanceAtStart: "150.00",
		StartedAt: now, LastActivity: now,
		DeviceType: "desktop", LobbyURL: "https://lobby",
		LaunchURL: "https://game", Token: "tok-1",
		Metadata: map[string]string{"player_id": "p1_abc"},
	}
}

func TestSessions_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	ctx := context.Background()

	if err := repo.CreateGameSession(ctx, sampleSession()); err != nil {
		t.Fatalf("dry-run CreateGameSession: %v", err)
	}
	// Empty balance normalizes to 0.
	s := sampleSession()
	s.BalanceAtStart = ""
	s.Metadata = nil
	if err := repo.CreateGameSession(ctx, s); err != nil {
		t.Fatalf("dry-run CreateGameSession defaults: %v", err)
	}
	if err := repo.CreateGameSession(ctx, nil); err == nil {
		t.Fatalf("nil session must fail")
	}
	if err := repo.CreateGameSession(ctx, &GameSession{}); err == nil {
		t.Fatalf("empty session id must fail")
	}
	bad := sampleSession()
	bad.BalanceAtStart = "NaN"
	if err := repo.CreateGameSession(ctx, bad); err == nil {
		t.Fatalf("invalid balance must fail")
	}

	if _, err := repo.GetGameSession(ctx, "s-1"); err != nil {
		t.Fatalf("dry-run GetGameSession: %v", err)
	}
	if _, _, err := repo.GetSessionsByUser(ctx, "u-1", 10, 0); err != nil {
		t.Fatalf("dry-run GetSessionsByUser: %v", err)
	}

	ended := time.Now()
	s2 := sampleSession()
	s2.EndedAt = &ended
	// Dry-run reports 0 rows affected -> not-found error path.
	if err := repo.UpdateGameSession(ctx, s2); err == nil {
		t.Fatalf("dry-run UpdateGameSession should report not found")
	}
	if err := repo.UpdateGameSession(ctx, nil); err == nil {
		t.Fatalf("nil session must fail")
	}
	if err := repo.UpdateGameSession(ctx, &GameSession{}); err == nil {
		t.Fatalf("empty session id must fail")
	}
	// Without EndedAt the update omits ended_at.
	if err := repo.UpdateGameSession(ctx, sampleSession()); err == nil {
		t.Fatalf("dry-run UpdateGameSession should report not found")
	}
	if err := repo.EndGameSession(ctx, "s-1", time.Now()); err == nil {
		t.Fatalf("dry-run EndGameSession should report not found")
	}
}

func TestHistory_DryRun(t *testing.T) {
	repo := NewCasinoRepository(openDryRunDB(t), nil)
	ctx := context.Background()

	gid := "g-1"
	now := time.Now()
	dr := &struct{ From, To time.Time }{From: now.Add(-time.Hour), To: now}
	if _, _, err := repo.GetGameHistory(ctx, "u-1", nil, nil, 10, 0); err != nil {
		t.Fatalf("dry-run GetGameHistory: %v", err)
	}
	if _, _, err := repo.GetGameHistory(ctx, "u-1", &gid, dr, 0, -1); err != nil {
		t.Fatalf("dry-run GetGameHistory filtered: %v", err)
	}
	if _, _, err := repo.GetRoundHistory(ctx, "s-1", 10, 0); err != nil {
		t.Fatalf("dry-run GetRoundHistory: %v", err)
	}

	now2 := time.Now()
	r := &GameRound{ID: "r-1", SessionID: "s-1", RoundID: "round-1",
		BetAmount: "10.00", WinAmount: "0", NetResult: "-10.00",
		Status: "completed", StartedAt: now2, EndedAt: now2,
		GameState: map[string]string{"reels": "7-7-7"}}
	if err := repo.CreateGameRound(ctx, r); err != nil {
		t.Fatalf("dry-run CreateGameRound: %v", err)
	}
	r2 := &GameRound{ID: "r-2", SessionID: "s-1", StartedAt: now2, EndedAt: now2}
	if err := repo.CreateGameRound(ctx, r2); err != nil {
		t.Fatalf("dry-run CreateGameRound defaults: %v", err)
	}
	if err := repo.CreateGameRound(ctx, nil); err == nil {
		t.Fatalf("nil round must fail")
	}
	if err := repo.CreateGameRound(ctx, &GameRound{}); err == nil {
		t.Fatalf("empty round id must fail")
	}
	for _, tc := range []struct {
		name  string
		mutate func(*GameRound)
	}{
		{"bad bet", func(r *GameRound) { r.BetAmount = "NaN" }},
		{"bad win", func(r *GameRound) { r.WinAmount = "NaN" }},
		{"bad net", func(r *GameRound) { r.NetResult = "NaN" }},
	} {
		rr := &GameRound{ID: "r-x", SessionID: "s-1", BetAmount: "1", WinAmount: "0",
			NetResult: "-1", StartedAt: now2, EndedAt: now2}
		tc.mutate(rr)
		if err := repo.CreateGameRound(ctx, rr); err == nil {
			t.Fatalf("%s must fail", tc.name)
		}
	}
}

// ─── Error branches (connection refused) ────────────────────────────────────

func TestDatabaseMethods_ReturnErrorWhenDatabaseDown(t *testing.T) {
	repo := NewCasinoRepository(openFailingDB(t), nil)
	ctx := context.Background()
	now := time.Now()

	cases := []struct {
		name string
		call func() error
	}{
		{"GetGames", func() error { _, _, err := repo.GetGames(ctx, GetGamesOptions{Limit: 5}); return err }},
		{"GetGame", func() error { _, err := repo.GetGame(ctx, "g-1"); return err }},
		{"UpsertGame", func() error { return repo.UpsertGame(ctx, sampleGame()) }},
		{"GetProviders", func() error { _, err := repo.GetProviders(ctx, nil); return err }},
		{"GetProvider", func() error { _, err := repo.GetProvider(ctx, "p-1"); return err }},
		{"UpsertProvider", func() error { return repo.UpsertProvider(ctx, &Provider{ID: "p"}) }},
		{"CreateGameSession", func() error { return repo.CreateGameSession(ctx, sampleSession()) }},
		{"GetGameSession", func() error { _, err := repo.GetGameSession(ctx, "s-1"); return err }},
		{"UpdateGameSession", func() error { return repo.UpdateGameSession(ctx, sampleSession()) }},
		{"EndGameSession", func() error { return repo.EndGameSession(ctx, "s-1", now) }},
		{"GetSessionsByUser", func() error { _, _, err := repo.GetSessionsByUser(ctx, "u-1", 5, 0); return err }},
		{"GetGameHistory", func() error { _, _, err := repo.GetGameHistory(ctx, "u-1", nil, nil, 5, 0); return err }},
		{"GetRoundHistory", func() error { _, _, err := repo.GetRoundHistory(ctx, "s-1", 5, 0); return err }},
		{"CreateGameRound", func() error {
			return repo.CreateGameRound(ctx, &GameRound{ID: "r-1", SessionID: "s-1", StartedAt: now, EndedAt: now})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatalf("expected error when database is down")
			}
		})
	}
}

// ─── Exec-success paths (fake driver, RowsAffected=1) ───────────────────────

func TestWriteMethods_FakeExecSuccess(t *testing.T) {
	repo := NewCasinoRepository(openFakeExecDB(t), nil)
	ctx := context.Background()

	if err := repo.CreateGameSession(ctx, sampleSession()); err != nil {
		t.Fatalf("CreateGameSession: %v", err)
	}
	if err := repo.UpdateGameSession(ctx, sampleSession()); err != nil {
		t.Fatalf("UpdateGameSession: %v", err)
	}
	if err := repo.EndGameSession(ctx, "s-1", time.Now()); err != nil {
		t.Fatalf("EndGameSession: %v", err)
	}
	if err := repo.CreateGameRound(ctx, &GameRound{ID: "r-1", SessionID: "s-1",
		BetAmount: "1", WinAmount: "0", NetResult: "-1",
		StartedAt: time.Now(), EndedAt: time.Now()}); err != nil {
		t.Fatalf("CreateGameRound: %v", err)
	}
	if err := repo.UpsertGame(ctx, sampleGame()); err != nil {
		t.Fatalf("UpsertGame: %v", err)
	}
	if err := repo.UpsertProvider(ctx, &Provider{ID: "p"}); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
}

// ─── PlayerMapper (same package) ────────────────────────────────────────────

func TestPlayerMapper_DryRun(t *testing.T) {
	m := NewPlayerMapper(openDryRunDB(t))
	ctx := context.Background()

	uid, err := m.GetInternalUserID(ctx, "pragmatic", "player-1")
	if err != nil || uid != 0 {
		t.Fatalf("dry-run resolve: uid=%d err=%v", uid, err)
	}
	pid, err := m.GetPlayerID(ctx, "pragmatic", 42)
	if err != nil {
		t.Fatalf("dry-run reverse: %v", err)
	}
	_ = pid
}

func TestPlayerMapper_Errors(t *testing.T) {
	m := NewPlayerMapper(openFailingDB(t))
	ctx := context.Background()

	if _, err := m.GetInternalUserID(ctx, "pragmatic", "player-1"); err == nil {
		t.Fatalf("expected resolve error when database is down")
	}
	if _, err := m.GetPlayerID(ctx, "pragmatic", 42); err == nil {
		t.Fatalf("expected create error when database is down")
	}
}

func TestPlayerMapper_CreateSuccess(t *testing.T) {
	m := NewPlayerMapper(openFakeExecDB(t))
	ctx := context.Background()

	pid, err := m.GetPlayerID(ctx, "pragmatic", 42)
	if err != nil {
		t.Fatalf("auto-create mapping: %v", err)
	}
	if pid != "p42_pra" {
		t.Fatalf("unexpected deterministic player id %q", pid)
	}
}

// ─── Redis cache against fake RESP server ───────────────────────────────────

func TestGameCache_RoundTrip(t *testing.T) {
	_, client := startFakeRedis(t)
	repo := NewCasinoRepository(nil, client)
	ctx := context.Background()

	game := sampleGame()
	if err := repo.CacheGame(ctx, game, time.Minute); err != nil {
		t.Fatalf("CacheGame: %v", err)
	}
	got, err := repo.GetCachedGame(ctx, "g-1")
	if err != nil {
		t.Fatalf("GetCachedGame: %v", err)
	}
	if got == nil || got.ID != "g-1" || got.Name != "Sweet Bonanza" {
		t.Fatalf("cache round-trip mismatch: %+v", got)
	}
	if len(got.Tags) != 2 || got.Features.BonusFeatures[0] != "multiplier" {
		t.Fatalf("complex fields must survive cache: %+v", got)
	}
	if err := repo.InvalidateGameCache(ctx, "g-1"); err != nil {
		t.Fatalf("InvalidateGameCache: %v", err)
	}
	got, err = repo.GetCachedGame(ctx, "g-1")
	if err != nil {
		t.Fatalf("GetCachedGame after invalidate: %v", err)
	}
	if got != nil {
		t.Fatalf("expected miss after invalidate")
	}
}

func TestGameCache_MissAndCorrupt(t *testing.T) {
	srv, client := startFakeRedis(t)
	repo := NewCasinoRepository(nil, client)
	ctx := context.Background()

	got, err := repo.GetCachedGame(ctx, "nope")
	if err != nil || got != nil {
		t.Fatalf("expected (nil,nil) on miss, got %+v err %v", got, err)
	}
	srv.plantRaw(GameCacheKey("bad"), "{not-json")
	if _, err := repo.GetCachedGame(ctx, "bad"); err == nil {
		t.Fatalf("expected unmarshal error on corrupt payload")
	}
}

func TestGameCache_ConnectionError(t *testing.T) {
	// Nothing listens on port 1 -> fast refused error, covers error lines.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", Protocol: 2})
	defer func() { _ = client.Close() }()
	repo := NewCasinoRepository(nil, client)
	ctx := context.Background()

	if err := repo.CacheGame(ctx, sampleGame(), time.Minute); err == nil {
		t.Fatalf("expected error when redis is down")
	}
	if _, err := repo.GetCachedGame(ctx, "g-1"); err == nil {
		t.Fatalf("expected error when redis is down")
	}
}
