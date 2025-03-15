#!/bin/bash

echo "🧹 Cleaning up project files..."

# Move old/backup files to archive
echo "Moving backup files to archive..."
mkdir -p archive/backups
find . -name "*.old" -o -name "*.bak" -o -name "*~" | xargs -I{} mv {} archive/backups/ 2>/dev/null || true

# Remove temporary files
echo "Removing temporary files..."
find . -name "*.tmp" -o -name "*.temp" | xargs rm -f 2>/dev/null || true

# Remove __pycache__ directories
echo "Removing Python cache files..."
find . -name "__pycache__" -type d | xargs rm -rf 2>/dev/null || true
find . -name "*.pyc" | xargs rm -f 2>/dev/null || true

# Clean up Docker artifacts
echo "Cleaning Docker artifacts..."
docker system prune -f > /dev/null 2>&1 || true

echo "✅ Cleanup completed!"