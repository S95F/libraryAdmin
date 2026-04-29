package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// ANSI colours
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	green   = "\x1b[32m"
	blue    = "\x1b[34m"
	yellow  = "\x1b[33m"
	red     = "\x1b[31m"
	cyan    = "\x1b[36m"
	magenta = "\x1b[35m"
)

func step(n int, s string)  { fmt.Printf("\n%s%s  Step %d  %s%s%s\n", bold, cyan, n, reset, bold, s+reset) }
func ok(s string)            { fmt.Printf("  %s✓%s  %s\n", green, reset, s) }
func fail(s string)          { fmt.Fprintf(os.Stderr, "  %s✗%s  %s\n", red, reset, s) }
func info(s string)          { fmt.Printf("  %s→%s  %s\n", blue, reset, s) }
func warn(s string)          { fmt.Printf("  %s⚠%s  %s\n", yellow, reset, s) }
func hr()                    { fmt.Printf("  %s%s%s\n", dim, strings.Repeat("─", 52), reset) }

var stdin = bufio.NewReader(os.Stdin)

func ask(question, defaultVal string) string {
	hint := ""
	if defaultVal != "" {
		hint = fmt.Sprintf(" %s[%s]%s", dim, defaultVal, reset)
	}
	fmt.Printf("  %s?%s  %s%s: ", blue, reset, question, hint)
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func askPassword(question string) string {
	fmt.Printf("  %s?%s  %s: ", blue, reset, question)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		// fallback: plain read (non-TTY)
		fmt.Printf("  %s?%s  %s: ", blue, reset, question)
		line, _ := stdin.ReadString('\n')
		return strings.TrimSpace(line)
	}
	return string(pw)
}

func confirm(question string) bool {
	ans := ask(fmt.Sprintf("%s %s(y/N)%s", question, dim, reset), "N")
	return strings.ToLower(strings.TrimSpace(ans)) == "y"
}

// runCmd executes a command and returns stdout, stderr, exit code.
func runCmd(name string, args []string, extraEnv map[string]string) (string, string, int) {
	cmd := exec.Command(name, args...)
	if extraEnv != nil {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			code = 1
		}
	}
	return out.String(), errBuf.String(), code
}

type config struct {
	host, port                    string
	adminUser, adminPassword      string
	database, user, password      string
	listenPort, sslMode           string
}

func psqlAdmin(cfg config, args []string) (string, string, int) {
	base := []string{
		"-h", cfg.host, "-p", cfg.port,
		"-U", cfg.adminUser, "-d", "postgres",
		"--no-password", "-v", "ON_ERROR_STOP=1",
	}
	return runCmd("psql", append(base, args...), map[string]string{"PGPASSWORD": cfg.adminPassword})
}

// ── Step 1 ────────────────────────────────────────────────────────────────────

func checkPrereqs() bool {
	step(1, "Checking prerequisites")

	if _, err := exec.LookPath("psql"); err != nil {
		fail("psql not found in PATH.")
		fail("Install PostgreSQL client tools: https://www.postgresql.org/download/")
		os.Exit(1)
	}
	stdout, _, _ := runCmd("psql", []string{"--version"}, nil)
	ok("psql  " + strings.TrimSpace(stdout))

	if _, err := exec.LookPath("go"); err != nil {
		warn("Go not found — server.exe will not be built.")
		info("Run migrations manually after copying a pre-built server.exe here.")
		return false
	}
	stdout, _, _ = runCmd("go", []string{"version"}, nil)
	ok("Go    " + strings.TrimSpace(strings.TrimPrefix(stdout, "go version ")))
	return true
}

// ── Step 2 ────────────────────────────────────────────────────────────────────

func promptConfig() config {
	step(2, "Database configuration")
	fmt.Printf("  %sPress Enter to accept the default shown in [brackets].%s\n", dim, reset)

	fmt.Println()
	hr()
	fmt.Printf("  %sAdmin connection%s  %s(needs CREATE ROLE / DATABASE rights)%s\n", bold, reset, dim, reset)
	host      := ask("PostgreSQL host", "localhost")
	port      := ask("PostgreSQL port", "5432")
	adminUser := ask("Admin username", "postgres")
	adminPass := askPassword("Admin password")

	fmt.Println()
	hr()
	fmt.Printf("  %sApplication database%s\n", bold, reset)
	database := ask("Database name", "library_db")
	user     := ask("App role name", "library_user")
	password := askPassword("App role password (min 8 chars)")
	if len(password) < 8 {
		fail("Password must be at least 8 characters.")
		os.Exit(1)
	}
	confirm2 := askPassword("Confirm password")
	if password != confirm2 {
		fail("Passwords do not match.")
		os.Exit(1)
	}

	fmt.Println()
	hr()
	fmt.Printf("  %sServer settings%s\n", bold, reset)
	listenPort := ask("HTTP listen port", "8080")
	sslMode    := ask("PG SSL mode  (disable/require/verify-full)", "disable")

	return config{
		host: host, port: port,
		adminUser: adminUser, adminPassword: adminPass,
		database: database, user: user, password: password,
		listenPort: listenPort, sslMode: sslMode,
	}
}

// ── Step 3 ────────────────────────────────────────────────────────────────────

func createDatabase(cfg config) {
	step(3, "Creating PostgreSQL role and database")

	if _, stderr, code := psqlAdmin(cfg, []string{"-c", "SELECT 1"}); code != 0 {
		fail("Admin connection failed:")
		fail(strings.TrimSpace(stderr))
		os.Exit(1)
	}
	ok("Admin connection verified")

	// Role
	stdout, _, _ := psqlAdmin(cfg, []string{"-tAc",
		fmt.Sprintf("SELECT 1 FROM pg_roles WHERE rolname='%s'", cfg.user)})
	if strings.TrimSpace(stdout) == "1" {
		warn(fmt.Sprintf("Role %q already exists — skipping creation", cfg.user))
	} else {
		escapedPw := strings.ReplaceAll(cfg.password, "'", "''")
		if _, stderr, code := psqlAdmin(cfg, []string{"-c",
			fmt.Sprintf(`CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`, cfg.user, escapedPw),
		}); code != 0 {
			fail("Failed to create role:"); fail(strings.TrimSpace(stderr)); os.Exit(1)
		}
		ok(fmt.Sprintf("Role %q created", cfg.user))
	}

	// Database
	stdout, _, _ = psqlAdmin(cfg, []string{"-tAc",
		fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", cfg.database)})
	if strings.TrimSpace(stdout) == "1" {
		warn(fmt.Sprintf("Database %q already exists", cfg.database))
		if !confirm("Continue and re-run migrations on the existing database?") {
			info("Aborted."); os.Exit(0)
		}
	} else {
		if _, stderr, code := psqlAdmin(cfg, []string{"-c",
			fmt.Sprintf(`CREATE DATABASE "%s" OWNER "%s"`, cfg.database, cfg.user),
		}); code != 0 {
			fail("Failed to create database:"); fail(strings.TrimSpace(stderr)); os.Exit(1)
		}
		ok(fmt.Sprintf("Database %q created (owner: %s)", cfg.database, cfg.user))
	}

	psqlAdmin(cfg, []string{"-c",
		fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, cfg.database, cfg.user)})
	ok("Privileges granted")
}

// ── Step 4 ────────────────────────────────────────────────────────────────────

func writeConfigFiles(cfg config) {
	step(4, "Writing configuration files")

	jwtBytes := make([]byte, 32)
	rand.Read(jwtBytes)
	jwt := hex.EncodeToString(jwtBytes)

	// .env
	envLines := []string{
		"# LibraryMS — auto-generated by installer",
		"PGHOST=" + cfg.host,
		"PGPORT=" + cfg.port,
		"PGDATABASE=" + cfg.database,
		"PGUSER=" + cfg.user,
		"PGPASSWORD=" + cfg.password,
		"PGSSLMODE=" + cfg.sslMode,
		"PORT=" + cfg.listenPort,
		"JWT_SECRET=" + jwt,
		"",
	}
	must(os.WriteFile(".env", []byte(strings.Join(envLines, "\n")), 0600), ".env")
	ok(".env")

	// setenv.bat
	batLines := []string{
		"@echo off",
		"REM LibraryMS environment — generated by installer",
		"SET PGHOST=" + cfg.host,
		"SET PGPORT=" + cfg.port,
		"SET PGDATABASE=" + cfg.database,
		"SET PGUSER=" + cfg.user,
		"SET PGPASSWORD=" + cfg.password,
		"SET PGSSLMODE=" + cfg.sslMode,
		"SET PORT=" + cfg.listenPort,
		"SET JWT_SECRET=" + jwt,
		"server.exe",
		"",
	}
	must(os.WriteFile("setenv.bat", []byte(strings.Join(batLines, "\r\n")), 0644), "setenv.bat")
	ok("setenv.bat")

	// setenv.ps1
	ps1Lines := []string{
		"# LibraryMS environment — generated by installer",
		"$env:PGHOST     = '" + cfg.host + "'",
		"$env:PGPORT     = '" + cfg.port + "'",
		"$env:PGDATABASE = '" + cfg.database + "'",
		"$env:PGUSER     = '" + cfg.user + "'",
		"$env:PGPASSWORD = '" + cfg.password + "'",
		"$env:PGSSLMODE  = '" + cfg.sslMode + "'",
		"$env:PORT       = '" + cfg.listenPort + "'",
		"$env:JWT_SECRET = '" + jwt + "'",
		"# Run: .\\server.exe",
		"",
	}
	must(os.WriteFile("setenv.ps1", []byte(strings.Join(ps1Lines, "\n")), 0644), "setenv.ps1")
	ok("setenv.ps1")

	// Export for child processes (migrations step)
	os.Setenv("PGHOST", cfg.host)
	os.Setenv("PGPORT", cfg.port)
	os.Setenv("PGDATABASE", cfg.database)
	os.Setenv("PGUSER", cfg.user)
	os.Setenv("PGPASSWORD", cfg.password)
	os.Setenv("PGSSLMODE", cfg.sslMode)
	os.Setenv("PORT", cfg.listenPort)
	os.Setenv("JWT_SECRET", jwt)
	ok("PG* vars exported to current process")
}

func must(err error, label string) {
	if err != nil {
		fail(fmt.Sprintf("Failed to write %s: %v", label, err))
		os.Exit(1)
	}
}

// ── Step 5 ────────────────────────────────────────────────────────────────────

func buildServer(goAvail bool) bool {
	step(5, "Building server binary")

	if !goAvail {
		warn("Go not installed — skipping build.")
		return false
	}

	info("go build -o server.exe . …")
	cmd := exec.Command("go", "build", "-o", "server.exe", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail("Build failed — see output above.")
		return false
	}
	ok("server.exe built")
	return true
}

// ── Step 6 ────────────────────────────────────────────────────────────────────

func runMigrations(serverBuilt bool) bool {
	step(6, "Running migrations")

	tryMigrate := func() bool {
		cmd := exec.Command("server.exe", "--migrate")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run() == nil
	}

	if serverBuilt {
		info("Executing server.exe --migrate …")
		if tryMigrate() {
			ok("Migrations complete")
			return true
		}
		fail("server.exe --migrate failed — see output above")
		return false
	}

	// Try an already-present server.exe
	if _, err := os.Stat("server.exe"); err == nil {
		info("Found existing server.exe — trying --migrate …")
		if tryMigrate() {
			ok("Migrations complete")
			return true
		}
	}

	fail("Cannot run migrations — server.exe not available.")
	info("Build with:  go build -o server.exe .")
	info("Then run:    server.exe --migrate")
	return false
}

// ── Summary ───────────────────────────────────────────────────────────────────

func printSummary(cfg config, serverBuilt bool) {
	line := strings.Repeat("═", 54)
	fmt.Println()
	fmt.Printf("  %s%s%s\n", green, line, reset)
	fmt.Printf("  %s%s  ✓  LibraryMS installed successfully!%s\n", green, bold, reset)
	fmt.Printf("  %s%s%s\n", green, line, reset)
	fmt.Println()
	fmt.Printf("  %sConnection%s   %s@%s:%s/%s\n", bold, reset, cfg.user, cfg.host, cfg.port, cfg.database)
	fmt.Printf("  %sDemo login%s   admin@library.com  /  Admin123!\n", bold, reset)
	fmt.Printf("              clerk@library.com  /  Clerk123!\n")
	fmt.Println()
	if serverBuilt {
		fmt.Printf("  %sStart server%s\n", bold, reset)
		fmt.Printf("    CMD:         %ssetenv.bat%s\n", cyan, reset)
		fmt.Printf("    PowerShell:  %s. .\\setenv.ps1 ; .\\server.exe%s\n", cyan, reset)
		fmt.Printf("    .env loaded: %s.\\server.exe%s   (reads .env automatically)\n", cyan, reset)
	} else {
		fmt.Printf("  %sserver.exe not built%s — build on a machine with Go ≥ 1.22:\n", bold, reset)
		fmt.Printf("    %sgo build -o server.exe .%s\n", cyan, reset)
		fmt.Printf("  Then run  %ssetenv.bat%s\n", cyan, reset)
	}
	fmt.Println()
	fmt.Printf("    %s%s→  http://localhost:%s%s\n", bold, cyan, cfg.listenPort, reset)
	fmt.Println()
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	fmt.Print("\x1Bc") // clear screen
	fmt.Println()
	fmt.Printf("%s%s  ╔══════════════════════════════════════════╗%s\n", bold, magenta, reset)
	fmt.Printf("%s%s  ║     LibraryMS  Installer  v1.0.0         ║%s\n", bold, magenta, reset)
	fmt.Printf("%s%s  ╚══════════════════════════════════════════╝%s\n", bold, magenta, reset)
	fmt.Println()

	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			fmt.Println("  Usage:  installer.exe")
			fmt.Println("  Flags:  --help   Show this message")
			fmt.Println()
			os.Exit(0)
		}
	}

	goAvail    := checkPrereqs()
	cfg        := promptConfig()
	createDatabase(cfg)
	writeConfigFiles(cfg)
	serverBuilt := buildServer(goAvail)
	migOk       := runMigrations(serverBuilt)

	if !migOk {
		fmt.Println()
		warn("Run migrations manually:  server.exe --migrate")
	}
	printSummary(cfg, serverBuilt)
}
