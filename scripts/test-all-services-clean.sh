#!/bin/bash

# Clean wrapper for test-all-services.sh that filters out shell environment noise
# This script runs the main test script and filters out known shell environment errors

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🧪 Running Orbit Messenger Services Test Suite..."
echo "=================================================="

# Run the main test script and filter out shell environment errors
"$SCRIPT_DIR/test-all-services.sh" 2>&1 | grep -v -E "(_encode:[0-9]+: command not found|setValueForKeyFakeAssocArray:[0-9]+: command not found)"

# Capture the exit code from the actual test script
exit_code=${PIPESTATUS[0]}

echo ""
if [ $exit_code -eq 0 ]; then
    echo "🎉 Test suite completed successfully!"
else
    echo "❌ Test suite failed with exit code: $exit_code"
fi

exit $exit_code
