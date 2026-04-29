#!/usr/bin/env node
/**
 * install.js — Interactive installer for LibraryMS
 *
 * Steps:
 *   1. Check prerequisites  (psql required, go optional)
 *   2. Prompt for admin PG credentials + new DB/role settings
 *   3. Create PostgreSQL role and database via psql
 *   4. Write  server/db.config.js
 *   5. Write  .env  +  setenv.bat  +  setenv.ps1
 *   6. Set PG* env vars in current process (for child processes)
 *   7. Build  server.exe  via  go build  (if Go ≥ 1.22 is installed)
 *   8. Run migrations:
 *        • server.exe --migrate      (preferred, if build succeeded)
 *        • node migrate.js --seed    (fallback — no Go required)
 */

'use strict';

const readline   = require('readline');
const { spawnSync } = require('child_process');
const fs         = require('fs');
const path       = require('path');
const os         = require('os');
const crypto     = require('crypto');

// ─── ANSI colours ─────────────────────────────────────────────────────────────
const c = {
  reset:   '\x1b[0m',
  bold:    '\x1b[1m',
  dim:     '\x1b[2m',
  green:   '\x1b[32m',
  blue:    '\x1b[34m',
  yellow:  '\x1b[33m',
  red:     '\x1b[31m',
  cyan:    '\x1b[36m',
  magenta: '\x1b[35m',
};

const step = (n, s) => { console.log(); console.log(`${c.bold}${c.cyan}  Step ${n}  ${c.reset}${c.bold}${s}${c.reset}`); };
const ok   = (s)    => console.log(`  ${c.green}✓${c.reset}  ${s}`);
const fail = (s)    => console.error(`  ${c.red}✗${c.reset}  ${s}`);
const info = (s)    => console.log(`  ${c.blue}→${c.reset}  ${s}`);
const warn = (s)    => console.log(`  ${c.yellow}⚠${c.reset}  ${s}`);
const hr   = ()     => console.log(`  ${c.dim}${'─'.repeat(52)}${c.reset}`);

// ─── Prompt helpers ───────────────────────────────────────────────────────────
function ask(rl, question, defaultVal) {
  return new Promise(resolve => {
    const hint = defaultVal ? ` ${c.dim}[${defaultVal}]${c.reset}` : '';
    rl.question(`  ${c.blue}?${c.reset}  ${question}${hint}: `, ans => {
      resolve(ans.trim() || defaultVal || '');
    });
  });
}

function askPassword(question) {
  return new Promise(resolve => {
    process.stdout.write(`  ${c.blue}?${c.reset}  ${question}: `);
    let pw = '';

    if (process.stdin.isTTY) {
      process.stdin.setRawMode(true);
      process.stdin.resume();
      process.stdin.setEncoding('utf8');

      const handler = (ch) => {
        switch (ch) {
          case '\r': case '\n':
            process.stdin.setRawMode(false);
            process.stdin.pause();
            process.stdin.removeListener('data', handler);
            process.stdout.write('\n');
            resolve(pw);
            break;
          case '\x7f': case '\b':
            if (pw.length) { pw = pw.slice(0,-1); process.stdout.write('\b \b'); }
            break;
          case '\x03':
            process.stdout.write('\n'); process.exit(1);
            break;
          default:
            pw += ch; process.stdout.write('*');
        }
      };
      process.stdin.on('data', handler);
    } else {
      // Non-TTY (piped / redirected stdin) — read plain
      const tmp = readline.createInterface({ input: process.stdin, terminal: false });
      tmp.once('line', line => { tmp.close(); resolve(line.trim()); });
    }
  });
}

async function confirm(rl, question) {
  const ans = await ask(rl, `${question} ${c.dim}(y/N)${c.reset}`, 'N');
  return ans.toLowerCase() === 'y';
}

// ─── Shell helpers ────────────────────────────────────────────────────────────
function which(cmd) {
  const r = spawnSync(os.platform() === 'win32' ? 'where' : 'which', [cmd],
    { encoding: 'utf8', stdio: 'pipe' });
  return r.status === 0;
}

function run(cmd, args, extraEnv = {}) {
  return spawnSync(cmd, args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, ...extraEnv },
  });
}

// ─── psql wrappers ────────────────────────────────────────────────────────────
// Run as admin user against 'postgres' maintenance DB
function psqlAdmin(cfg, args) {
  return run('psql', [
    '-h', cfg.host, '-p', String(cfg.port),
    '-U', cfg.adminUser, '-d', 'postgres',
    '--no-password', '-v', 'ON_ERROR_STOP=1',
    ...args,
  ], { PGPASSWORD: cfg.adminPassword });
}

// ─── Step 1 ───────────────────────────────────────────────────────────────────
function checkPrereqs() {
  step(1, 'Checking prerequisites');

  if (!which('psql')) {
    fail('psql not found in PATH.');
    fail('Install PostgreSQL client tools: https://www.postgresql.org/download/');
    process.exit(1);
  }
  ok(`psql  ${run('psql',['--version']).stdout.trim()}`);
  ok(`Node  ${process.version}`);

  let goAvailable = false;
  if (which('go')) {
    const goVer = run('go', ['version']).stdout.trim().replace('go version ', '');
    ok(`Go    ${goVer}`);
    goAvailable = true;
  } else {
    warn('Go not found — server.exe will not be built.');
    info('Migrations will run via  node migrate.js  instead.');
  }
  return { goAvailable };
}

// ─── Step 2 ───────────────────────────────────────────────────────────────────
async function promptConfig(rl) {
  step(2, 'Database configuration');
  console.log(`  ${c.dim}Press Enter to accept the default shown in [brackets].${c.reset}`);

  console.log();
  hr();
  console.log(`  ${c.bold}Admin connection${c.reset}  ${c.dim}(used only during setup — needs CREATE ROLE/DATABASE rights)${c.reset}`);
  const host      = await ask(rl, 'PostgreSQL host',  'localhost');
  const port      = await ask(rl, 'PostgreSQL port',  '5432');
  const adminUser = await ask(rl, 'Admin username',   'postgres');
  const adminPass = await askPassword('Admin password');

  console.log();
  hr();
  console.log(`  ${c.bold}Application database${c.reset}`);
  const database = await ask(rl, 'Database name',    'library_db');
  const user     = await ask(rl, 'App role name',    'library_user');
  const password = await askPassword('App role password (min 8 chars)');
  if (password.length < 8) { fail('Password must be at least 8 characters.'); process.exit(1); }
  const confirm2 = await askPassword('Confirm password');
  if (password !== confirm2)  { fail('Passwords do not match.'); process.exit(1); }

  console.log();
  hr();
  console.log(`  ${c.bold}Server settings${c.reset}`);
  const listenPort = await ask(rl, 'HTTP listen port',                           '8080');
  const sslMode    = await ask(rl, 'PG SSL mode  (disable/require/verify-full)', 'disable');

  return { host, port, adminUser, adminPassword: adminPass,
           database, user, password, listenPort, sslMode };
}

// ─── Step 3 ───────────────────────────────────────────────────────────────────
async function createDatabase(rl, cfg) {
  step(3, 'Creating PostgreSQL role and database');

  // Test admin connection
  const ping = psqlAdmin(cfg, ['-c', 'SELECT 1']);
  if (ping.status !== 0) {
    fail('Admin connection failed:');
    fail(ping.stderr || 'Authentication error or server unreachable');
    process.exit(1);
  }
  ok('Admin connection verified');

  // ── Role ─────────────────────────────────────────────────────────────────
  const roleRow = psqlAdmin(cfg, ['-tAc', `SELECT 1 FROM pg_roles WHERE rolname='${cfg.user}'`]);
  if (roleRow.stdout.trim() === '1') {
    warn(`Role "${cfg.user}" already exists — skipping creation`);
  } else {
    const escapedPw = cfg.password.replace(/'/g, "''");
    const r = psqlAdmin(cfg, ['-c', `CREATE ROLE "${cfg.user}" WITH LOGIN PASSWORD '${escapedPw}'`]);
    if (r.status !== 0) { fail('Failed to create role:'); fail(r.stderr); process.exit(1); }
    ok(`Role "${cfg.user}" created`);
  }

  // ── Database ─────────────────────────────────────────────────────────────
  const dbRow = psqlAdmin(cfg, ['-tAc', `SELECT 1 FROM pg_database WHERE datname='${cfg.database}'`]);
  if (dbRow.stdout.trim() === '1') {
    warn(`Database "${cfg.database}" already exists`);
    const proceed = await confirm(rl, 'Continue and re-run migrations on the existing database?');
    if (!proceed) { info('Aborted.'); process.exit(0); }
  } else {
    const r = psqlAdmin(cfg, ['-c', `CREATE DATABASE "${cfg.database}" OWNER "${cfg.user}"`]);
    if (r.status !== 0) { fail('Failed to create database:'); fail(r.stderr); process.exit(1); }
    ok(`Database "${cfg.database}" created (owner: ${cfg.user})`);
  }

  // Ensure privileges
  psqlAdmin(cfg, ['-c', `GRANT ALL PRIVILEGES ON DATABASE "${cfg.database}" TO "${cfg.user}"`]);
  ok('Privileges granted');
}

// ─── Step 4 + 5 ──────────────────────────────────────────────────────────────
function writeConfigFiles(cfg) {
  step(4, 'Writing configuration files');

  // Guarantee server/ directory exists
  fs.mkdirSync(path.join(__dirname, 'server'), { recursive: true });

  // server/db.config.js
  fs.writeFileSync(
    path.join(__dirname, 'server', 'db.config.js'),
    `// Auto-generated by install.js\nmodule.exports = {\n` +
    `  host:     '${cfg.host}',\n` +
    `  port:     ${cfg.port},\n` +
    `  database: '${cfg.database}',\n` +
    `  user:     '${cfg.user}',\n` +
    `  password: '${cfg.password.replace(/\\/g,'\\\\').replace(/'/g,"\\'")}',\n` +
    `  sslMode:  '${cfg.sslMode}',\n};\n`,
    'utf8',
  );
  ok('server/db.config.js');

  const jwtSecret = crypto.randomBytes(32).toString('hex');

  // .env  (Go server reads this via db.LoadDotEnv)
  fs.writeFileSync(path.join(__dirname, '.env'), [
    '# LibraryMS — auto-generated by install.js',
    `PGHOST=${cfg.host}`,
    `PGPORT=${cfg.port}`,
    `PGDATABASE=${cfg.database}`,
    `PGUSER=${cfg.user}`,
    `PGPASSWORD=${cfg.password}`,
    `PGSSLMODE=${cfg.sslMode}`,
    `PORT=${cfg.listenPort}`,
    `JWT_SECRET=${jwtSecret}`,
    '',
  ].join('\n'), 'utf8');
  ok('.env');

  // setenv.bat  (CMD — launches server)
  fs.writeFileSync(path.join(__dirname, 'setenv.bat'), [
    '@echo off',
    'REM LibraryMS environment — generated by install.js',
    `SET PGHOST=${cfg.host}`,
    `SET PGPORT=${cfg.port}`,
    `SET PGDATABASE=${cfg.database}`,
    `SET PGUSER=${cfg.user}`,
    `SET PGPASSWORD=${cfg.password}`,
    `SET PGSSLMODE=${cfg.sslMode}`,
    `SET PORT=${cfg.listenPort}`,
    `SET JWT_SECRET=${jwtSecret}`,
    'server.exe',
    '',
  ].join('\r\n'), 'utf8');
  ok('setenv.bat');

  // setenv.ps1  (PowerShell)
  fs.writeFileSync(path.join(__dirname, 'setenv.ps1'), [
    '# LibraryMS environment — generated by install.js',
    `$env:PGHOST     = '${cfg.host}'`,
    `$env:PGPORT     = '${cfg.port}'`,
    `$env:PGDATABASE = '${cfg.database}'`,
    `$env:PGUSER     = '${cfg.user}'`,
    `$env:PGPASSWORD = '${cfg.password}'`,
    `$env:PGSSLMODE  = '${cfg.sslMode}'`,
    `$env:PORT       = '${cfg.listenPort}'`,
    `$env:JWT_SECRET = '${jwtSecret}'`,
    '# Run: .\\server.exe',
    '',
  ].join('\n'), 'utf8');
  ok('setenv.ps1');

  // Apply to the current process so child processes inherit them
  Object.assign(process.env, {
    PGHOST: cfg.host, PGPORT: String(cfg.port), PGDATABASE: cfg.database,
    PGUSER: cfg.user, PGPASSWORD: cfg.password, PGSSLMODE: cfg.sslMode,
    PORT: String(cfg.listenPort), JWT_SECRET: jwtSecret,
  });
  ok('PG* vars exported to current process');
}

// ─── Step 6 ───────────────────────────────────────────────────────────────────
function buildServer(goAvailable) {
  step(5, 'Building server binary');

  if (!goAvailable) {
    warn('Go not installed — skipping build.');
    return false;
  }

  info('go build -o server.exe . …');
  const r = spawnSync('go', ['build', '-o', 'server.exe', '.'], {
    cwd:      __dirname,
    encoding: 'utf8',
    stdio:    ['ignore', 'pipe', 'pipe'],
    env:      process.env,
  });

  if (r.status !== 0) {
    fail('Build failed:');
    (r.stderr || r.stdout || '').split('\n').filter(Boolean).forEach(l => fail(`  ${l}`));
    warn('Falling back to  node migrate.js  for migrations.');
    return false;
  }

  ok('server.exe built');
  return true;
}

// ─── Step 7 ───────────────────────────────────────────────────────────────────
function runMigrations(cfg, serverBuilt) {
  step(6, 'Running migrations');

  if (serverBuilt) {
    info('Executing  server.exe --migrate …');
    const r = spawnSync(path.join(__dirname, 'server.exe'), ['--migrate'], {
      cwd: __dirname, encoding: 'utf8', stdio: 'inherit', env: process.env,
    });
    if (r.status === 0) { ok('Migrations complete (via server.exe --migrate)'); return true; }
    warn('server.exe --migrate returned non-zero — falling back to node migrate.js');
  }

  const migrateScript = path.join(__dirname, 'migrate.js');
  if (!fs.existsSync(migrateScript)) {
    fail('migrate.js not found. Run manually: node migrate.js --seed');
    return false;
  }
  info('Executing  node migrate.js --seed …');
  const r2 = spawnSync(process.execPath, [migrateScript, '--seed'], {
    cwd: __dirname, encoding: 'utf8', stdio: 'inherit', env: process.env,
  });
  if (r2.status !== 0) { fail('Migrations failed — see output above.'); return false; }
  ok('Migrations complete (via node migrate.js)');
  return true;
}

// ─── Summary ─────────────────────────────────────────────────────────────────
function printSummary(cfg, serverBuilt) {
  console.log();
  console.log(`  ${c.green}${'═'.repeat(54)}${c.reset}`);
  console.log(`  ${c.green}${c.bold}  ✓  LibraryMS installed successfully!${c.reset}`);
  console.log(`  ${c.green}${'═'.repeat(54)}${c.reset}`);
  console.log();
  console.log(`  ${c.bold}Connection${c.reset}   ${cfg.user}@${cfg.host}:${cfg.port}/${cfg.database}`);
  console.log(`  ${c.bold}Demo login${c.reset}   admin@library.com  /  Admin123!`);
  console.log(`              clerk@library.com  /  Clerk123!`);
  console.log();
  if (serverBuilt) {
    console.log(`  ${c.bold}Start server${c.reset}`);
    console.log(`    CMD:         ${c.cyan}setenv.bat${c.reset}`);
    console.log(`    PowerShell:  ${c.cyan}. .\\setenv.ps1 ; .\\server.exe${c.reset}`);
    console.log(`    .env loaded: ${c.cyan}.\\server.exe${c.reset}   (reads .env automatically)`);
  } else {
    console.log(`  ${c.bold}server.exe not built${c.reset} — build on a machine with Go ≥ 1.22:`);
    console.log(`    ${c.cyan}go build -o server.exe .${c.reset}`);
    console.log(`  Then copy here and run  ${c.cyan}setenv.bat${c.reset}`);
  }
  console.log();
  console.log(`    ${c.bold}${c.cyan}→  http://localhost:${cfg.listenPort}${c.reset}`);
  console.log();
}

// ─── Entry point ─────────────────────────────────────────────────────────────
(async () => {
  process.stdout.write('\x1Bc'); // clear screen
  console.log();
  console.log(`${c.bold}${c.magenta}  ╔══════════════════════════════════════════╗${c.reset}`);
  console.log(`${c.bold}${c.magenta}  ║     LibraryMS  Installer  v1.0.0         ║${c.reset}`);
  console.log(`${c.bold}${c.magenta}  ╚══════════════════════════════════════════╝${c.reset}`);
  console.log();

  if (process.argv.includes('--help') || process.argv.includes('-h')) {
    console.log('  Usage:  node install.js');
    console.log('  Flags:  --help   Show this message');
    console.log();
    process.exit(0);
  }

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: true });
  process.on('SIGINT', () => { rl.close(); console.log('\n  Cancelled.'); process.exit(1); });

  try {
    const { goAvailable } = checkPrereqs();
    const cfg = await promptConfig(rl);
    await createDatabase(rl, cfg);
    rl.close();

    writeConfigFiles(cfg);
    const serverBuilt = buildServer(goAvailable);
    const migOk       = runMigrations(cfg, serverBuilt);

    if (!migOk) {
      console.log();
      warn('Run migrations manually:  node migrate.js --seed');
    }
    printSummary(cfg, serverBuilt);
    process.exit(0);
  } catch (e) {
    rl.close();
    console.log();
    fail(`Unexpected error: ${e.message}`);
    if (process.env.DEBUG) console.error(e.stack);
    process.exit(1);
  }
})();
