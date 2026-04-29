#!/usr/bin/env node
/**
 * migrate.js — standalone migration runner (no Go required)
 *
 * Reads connection info from (in priority order):
 *   1. Environment variables  PGHOST / PGPORT / PGDATABASE / PGUSER / PGPASSWORD
 *   2. server/db.config.js   (written by install.js)
 *
 * Runs every SQL file in ./migrations/ in filename order via psql.
 * Safe to re-run (all statements use IF NOT EXISTS / CREATE INDEX IF NOT EXISTS).
 *
 * Usage:
 *   node migrate.js
 *   node migrate.js --seed       # also insert demo data
 *   node migrate.js --dry-run    # print SQL without executing
 */

'use strict';

const { execSync, spawnSync } = require('child_process');
const fs   = require('fs');
const path = require('path');

// ─── ANSI helpers ─────────────────────────────────────────────────────────────
const c = {
  reset:  '\x1b[0m',
  bold:   '\x1b[1m',
  green:  '\x1b[32m',
  blue:   '\x1b[34m',
  yellow: '\x1b[33m',
  red:    '\x1b[31m',
  cyan:   '\x1b[36m',
  dim:    '\x1b[2m',
};
const ok  = (s) => console.log(`  ${c.green}✓${c.reset} ${s}`);
const err = (s) => console.error(`  ${c.red}✗${c.reset} ${s}`);
const info= (s) => console.log(`  ${c.blue}→${c.reset} ${s}`);
const warn= (s) => console.log(`  ${c.yellow}⚠${c.reset} ${s}`);

// ─── Args ─────────────────────────────────────────────────────────────────────
const args    = process.argv.slice(2);
const dryRun  = args.includes('--dry-run');
const doSeed  = args.includes('--seed');

// ─── Config resolution ────────────────────────────────────────────────────────
function loadConfig() {
  // Start from environment
  const cfg = {
    host:     process.env.PGHOST     || 'localhost',
    port:     process.env.PGPORT     || '5432',
    database: process.env.PGDATABASE || 'library_db',
    user:     process.env.PGUSER     || 'library_user',
    password: process.env.PGPASSWORD || '',
  };

  // Overlay with server/db.config.js if present and env vars are still defaults
  const cfgFile = path.join(__dirname, 'server', 'db.config.js');
  if (fs.existsSync(cfgFile)) {
    try {
      const fileCfg = require(cfgFile);
      if (!process.env.PGHOST)     cfg.host     = fileCfg.host     || cfg.host;
      if (!process.env.PGPORT)     cfg.port     = String(fileCfg.port || cfg.port);
      if (!process.env.PGDATABASE) cfg.database = fileCfg.database || cfg.database;
      if (!process.env.PGUSER)     cfg.user     = fileCfg.user     || cfg.user;
      if (!process.env.PGPASSWORD) cfg.password = fileCfg.password || cfg.password;
    } catch (e) {
      warn(`Could not parse server/db.config.js: ${e.message}`);
    }
  }
  return cfg;
}

// ─── psql runner ─────────────────────────────────────────────────────────────
function psql(cfg, args) {
  const env = {
    ...process.env,
    PGPASSWORD: cfg.password,
  };
  const baseArgs = [
    '-h', cfg.host,
    '-p', String(cfg.port),
    '-U', cfg.user,
    '-d', cfg.database,
    '--no-password',
    '-v', 'ON_ERROR_STOP=1',
  ];
  return spawnSync('psql', [...baseArgs, ...args], {
    env,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

function psqlCommand(cfg, sql) {
  return psql(cfg, ['-c', sql]);
}

function psqlFile(cfg, filePath) {
  return psql(cfg, ['-f', filePath]);
}

function checkPsql() {
  const r = spawnSync('psql', ['--version'], { encoding: 'utf8', stdio: 'pipe' });
  if (r.error || r.status !== 0) {
    err('psql not found in PATH. Install PostgreSQL client tools.');
    process.exit(1);
  }
  return r.stdout.trim();
}

// ─── Connection test ──────────────────────────────────────────────────────────
function testConnection(cfg) {
  const r = psqlCommand(cfg, 'SELECT 1');
  if (r.status !== 0) {
    err(`Cannot connect to PostgreSQL:`);
    err(r.stderr || 'Unknown error');
    err(`Connection: ${cfg.user}@${cfg.host}:${cfg.port}/${cfg.database}`);
    return false;
  }
  return true;
}

// ─── Seed data ────────────────────────────────────────────────────────────────
// Embedded seed SQL so migrate.js works without any external files
const SEED_SQL = `
DO $$
DECLARE v_count INTEGER;
BEGIN
  SELECT COUNT(*) INTO v_count FROM users;
  IF v_count > 0 THEN
    RAISE NOTICE 'Seed data already present — skipping.';
    RETURN;
  END IF;

  -- Passwords are bcrypt of: Admin123!, Clerk123!, User123!
  INSERT INTO users (id, username, email, password_hash, role) VALUES
    (gen_random_uuid()::text, 'admin', 'admin@library.com',
     '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin'),
    (gen_random_uuid()::text, 'clerk', 'clerk@library.com',
     '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'clerk'),
    (gen_random_uuid()::text, 'alice', 'alice@example.com',
     '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'user');

  INSERT INTO books (id, isbn, title, author, genre, description, published_year, total_copies, available_copies) VALUES
    (gen_random_uuid()::text,'9780743273565','The Great Gatsby','F. Scott Fitzgerald','Fiction','A story of the fabulously wealthy Jay Gatsby.',1925,3,3),
    (gen_random_uuid()::text,'9780061935466','To Kill a Mockingbird','Harper Lee','Fiction','Racial injustice and the loss of innocence in the American South.',1960,4,4),
    (gen_random_uuid()::text,'9780451524935','1984','George Orwell','Dystopia','A dystopian novel set in a totalitarian society.',1949,5,5),
    (gen_random_uuid()::text,'9780441013593','Dune','Frank Herbert','Sci-Fi','Epic science fiction in a distant feudal future.',1965,4,4),
    (gen_random_uuid()::text,'9780345391803','The Hitchhiker''s Guide to the Galaxy','Douglas Adams','Sci-Fi','Comic science fiction following Arthur Dent.',1979,3,3),
    (gen_random_uuid()::text,'9780132350884','Clean Code','Robert C. Martin','Technology','A handbook of agile software craftsmanship.',2008,2,2),
    (gen_random_uuid()::text,'9780135957059','The Pragmatic Programmer','David Thomas','Technology','Your journey to mastery in software development.',1999,2,2),
    (gen_random_uuid()::text,'9780201633610','Design Patterns','Gang of Four','Technology','Elements of reusable object-oriented software.',1994,2,2),
    (gen_random_uuid()::text,'9780062316097','Sapiens','Yuval Noah Harari','History','A brief history of humankind.',2011,3,3),
    (gen_random_uuid()::text,'9780062315007','The Alchemist','Paulo Coelho','Fiction','A philosophical novel about pursuing dreams.',1988,5,5),
    (gen_random_uuid()::text,'9780060850524','Brave New World','Aldous Huxley','Dystopia','A dystopian vision of a future totalitarian state.',1932,3,3),
    (gen_random_uuid()::text,'9781590302255','The Art of War','Sun Tzu','Philosophy','An ancient Chinese military treatise on strategy.',-500,4,4),
    (gen_random_uuid()::text,'9780553380163','A Brief History of Time','Stephen Hawking','Science','Exploring the nature of time and the universe.',1988,3,3),
    (gen_random_uuid()::text,'9780374533557','Thinking, Fast and Slow','Daniel Kahneman','Psychology','The two systems that drive the way we think.',2011,3,3),
    (gen_random_uuid()::text,'9780316769488','The Catcher in the Rye','J.D. Salinger','Fiction','A coming-of-age story about teenage alienation.',1951,3,3),
    (gen_random_uuid()::text,'9780590353427','Harry Potter and the Sorcerer''s Stone','J.K. Rowling','Fantasy','The first book in the beloved Harry Potter series.',1997,6,6),
    (gen_random_uuid()::text,'9780618640157','The Lord of the Rings','J.R.R. Tolkien','Fantasy','An epic high-fantasy adventure in Middle-earth.',1954,4,4),
    (gen_random_uuid()::text,'9780486415871','Crime and Punishment','Fyodor Dostoevsky','Fiction','A psychological novel exploring guilt and redemption.',1866,2,2),
    (gen_random_uuid()::text,'9780140432053','The Origin of Species','Charles Darwin','Science','On the origin of species by natural selection.',1859,2,2),
    (gen_random_uuid()::text,'9780735211292','Atomic Habits','James Clear','Self-Help','A framework for building good habits.',2018,4,4);

  RAISE NOTICE 'Seed complete. Credentials: admin@library.com / Admin123!  |  clerk@library.com / Clerk123!';
END $$;
`;

// ─── Main ─────────────────────────────────────────────────────────────────────
async function main() {
  console.log();
  console.log(`${c.bold}${c.cyan}  LibraryMS — Database Migration Runner${c.reset}`);
  console.log(`  ${'─'.repeat(42)}`);
  console.log();

  if (dryRun) warn('DRY-RUN mode — no SQL will be executed\n');

  // 1. Check psql
  const psqlVer = checkPsql();
  ok(`psql found: ${psqlVer}`);

  // 2. Load config
  const cfg = loadConfig();
  info(`Connecting as ${c.bold}${cfg.user}${c.reset}@${cfg.host}:${cfg.port}/${c.bold}${cfg.database}${c.reset}`);

  if (dryRun) {
    console.log();
    info('Migration files that would run:');
    getMigrationFiles().forEach(f => console.log(`     ${f}`));
    if (doSeed) info('Seed SQL would also run.');
    console.log();
    process.exit(0);
  }

  // 3. Test connection
  if (!testConnection(cfg)) process.exit(1);
  ok('Connection successful');

  // 4. Run migration files
  const files = getMigrationFiles();
  if (!files.length) {
    warn('No migration files found in ./migrations/');
  } else {
    console.log();
    console.log(`  ${c.bold}Running ${files.length} migration file(s)…${c.reset}`);
    for (const file of files) {
      const filePath = path.join(__dirname, 'migrations', file);
      info(`Applying ${file}…`);
      const r = psqlFile(cfg, filePath);
      if (r.status !== 0) {
        err(`Failed on ${file}:`);
        err(r.stderr || 'Unknown error');
        process.exit(1);
      }
      ok(`${file} applied`);
    }
  }

  // 5. Optional seed
  if (doSeed) {
    console.log();
    info('Running seed data…');
    const r = psqlCommand(cfg, SEED_SQL);
    if (r.status !== 0) {
      err('Seed failed:');
      err(r.stderr || 'Unknown error');
      process.exit(1);
    }
    ok('Seed complete');
    // Print any NOTICEs
    if (r.stdout) {
      r.stdout.split('\n')
        .filter(l => l.includes('NOTICE'))
        .forEach(l => info(l.replace(/.*NOTICE:?\s*/i, '')));
    }
  }

  console.log();
  ok(`${c.bold}Migration finished successfully.${c.reset}`);
  console.log();
}

function getMigrationFiles() {
  const dir = path.join(__dirname, 'migrations');
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir)
    .filter(f => f.endsWith('.sql'))
    .sort();
}

main().catch(e => {
  err(e.message);
  process.exit(1);
});
