#!/bin/bash

echo "🔒 Fixing security issues identified in security check..."

# Fix .gitignore for secure credentials
echo "Fixing .gitignore for secure credentials..."
if ! grep -q "secure_credentials.txt" .gitignore; then
  echo "secure_credentials.txt" >> .gitignore
  echo "✅ Added secure_credentials.txt to .gitignore"
else
  echo "✓ secure_credentials.txt already in .gitignore"
fi

# Remove tracked secure credentials
echo "Removing tracked secure credentials..."
if git ls-files | grep -q "secure_credentials.txt"; then
  git rm --cached secure_credentials.txt
  echo "✅ Removed secure_credentials.txt from git tracking"
else 
  echo "✓ secure_credentials.txt not tracked in git"
fi

# Fix image pull policy in deployments
echo "Setting imagePullPolicy: Never in deployments..."
for file in $(find ./k8s -name "*deployment*.yaml"); do
  if ! grep -q "imagePullPolicy: Never" "$file"; then
    # Different sed syntax for macOS vs Linux
    if [[ "$(uname)" == "Darwin" ]]; then
      sed -i '' '/image:/a\
        imagePullPolicy: Never' "$file"
    else
      sed -i '/image:/a\        imagePullPolicy: Never' "$file"
    fi
    echo "✅ Added imagePullPolicy: Never to $file"
  else
    echo "✓ $file already has imagePullPolicy: Never"
  fi
done

# Fix Dockerfiles to use non-root users
echo "Fixing Dockerfiles to use non-root users..."
for file in $(find . -name "Dockerfile"); do
  if ! grep -q "USER" "$file"; then
    echo -e "\n# Run as non-root user\nUSER 1000" >> "$file"
    echo "✅ Added USER 1000 to $file"
  else
    echo "✓ $file already has USER instruction"
  fi
done

echo ""
echo "✅ Security fixes applied. Run security-check.sh again to verify."