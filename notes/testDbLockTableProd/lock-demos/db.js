'use strict';

const { Client } = require('pg');

function createClient() {
  return new Client({
    host: process.env.PGHOST || 'localhost',
    port: Number(process.env.PGPORT || 5432),
    user: process.env.PGUSER || 'app',
    password: process.env.PGPASSWORD || 'app',
    database: process.env.PGDATABASE || 'lock_lab',
  });
}

module.exports = { createClient };
