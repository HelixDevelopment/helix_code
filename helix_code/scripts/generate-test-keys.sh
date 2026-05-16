#!/bin/bash

# Generate SSH keys for test environment
set -e

echo "🔑 Generating SSH keys for test environment..."

# Create directories
mkdir -p test/workers/ssh-keys
mkdir -p test/config

# Generate SSH key pair for test workers
ssh-keygen -t rsa -b 4096 -f test/workers/ssh-keys/id_rsa -N "" -q

# Copy public key to authorized_keys
cp test/workers/ssh-keys/id_rsa.pub test/workers/ssh-keys/authorized_keys

# Set proper permissions
chmod 600 test/workers/ssh-keys/id_rsa
chmod 644 test/workers/ssh-keys/id_rsa.pub
chmod 644 test/workers/ssh-keys/authorized_keys

echo "✅ SSH keys generated successfully!"
echo "   Private key: test/workers/ssh-keys/id_rsa"
echo "   Public key:  test/workers/ssh-keys/id_rsa.pub"
echo ""
echo "📋 To use these keys in tests:"
echo "   - Private key path: test/workers/ssh-keys/id_rsa"
echo "   - Username: root"
echo "   - Password: testpassword"
echo ""
echo "🚀 You can now run the test environment with:"
echo "   docker-compose -f docker-compose.test.yml up --build"