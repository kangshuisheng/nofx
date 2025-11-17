#!/bin/bash
set -e

# Migrate config.db and trading.db from project root into data/ for persistence
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$PROJECT_ROOT"

mkdir -p data
chmod 755 data

if [ -f "config.db" ] && [ ! -f "data/config.db" ]; then
  echo "Migrating config.db -> data/config.db"
  mv config.db data/config.db
  chmod 600 data/config.db
  echo "Done: data/config.db"
fi

if [ -f "trading.db" ] && [ ! -f "data/trading.db" ]; then
  echo "Migrating trading.db -> data/trading.db"
  mv trading.db data/trading.db
  chmod 600 data/trading.db
  echo "Done: data/trading.db"
fi

echo "Migration complete."