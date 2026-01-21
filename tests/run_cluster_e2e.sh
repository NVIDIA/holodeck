#!/bin/bash
# E2E Cluster Test Runner
# Usage: ./run_cluster_e2e.sh [test-label]
#
# Test labels (maps to Ginkgo labels):
#   minimal     - 1 CP + 1 worker (smallest, fastest)
#   ha          - 3 dedicated CP nodes + 2 GPU workers (HA)
#   dedicated   - 1 dedicated CPU CP + 3 GPU workers
#   gpu         - Tests with GPU instances (3gpu or dedicated or ha)
#   cluster     - All cluster tests (default)

set -e

# Configuration - EDIT THESE VALUES
export AWS_SSH_KEY_NAME="${AWS_SSH_KEY_NAME:-cnt-ci}"
export E2E_SSH_KEY="${E2E_SSH_KEY:-$HOME/.ssh/cnt-ci.pem}"
export LOG_ARTIFACT_DIR="${LOG_ARTIFACT_DIR:-/tmp/holodeck-e2e}"

# Validate configuration
echo "=== E2E Cluster Test Configuration ==="
echo "AWS_SSH_KEY_NAME: $AWS_SSH_KEY_NAME"
echo "E2E_SSH_KEY:      $E2E_SSH_KEY"
echo "LOG_ARTIFACT_DIR: $LOG_ARTIFACT_DIR"
echo ""

if [ ! -f "$E2E_SSH_KEY" ]; then
    echo "ERROR: SSH key not found at $E2E_SSH_KEY"
    echo "Please set E2E_SSH_KEY to your SSH private key path"
    exit 1
fi

if ! aws sts get-caller-identity &>/dev/null; then
    echo "ERROR: AWS credentials not configured"
    echo "Please run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY"
    exit 1
fi

# Create artifact directory
mkdir -p "$LOG_ARTIFACT_DIR"

# Determine which test(s) to run
TEST_LABEL="${1:-all}"

case "$TEST_LABEL" in
    "minimal"|"ha"|"dedicated"|"gpu"|"multinode"|"cluster")
        LABEL_FILTER="--ginkgo.label-filter=$TEST_LABEL"
        ;;
    "all")
        LABEL_FILTER="--ginkgo.label-filter=cluster"
        ;;
    *)
        echo "Unknown test label: $TEST_LABEL"
        echo "Valid labels: minimal, ha, dedicated, gpu, multinode, cluster, all"
        exit 1
        ;;
esac

echo "=== Running E2E tests with label: $TEST_LABEL ==="
echo ""

cd "$(dirname "$0")/.."

# Run the tests
# Note: -test.timeout must be >= ginkgo.timeout to avoid premature test termination
# Timeouts: HA clusters with 5 nodes can take 20-25 min each, so allow ~2.5 hours for full suite
go test -v ./tests/... \
    -test.timeout=180m \
    --ginkgo.v \
    "$LABEL_FILTER" \
    --ginkgo.timeout=150m \
    --ginkgo.fail-fast

echo ""
echo "=== E2E tests completed ==="
echo "Artifacts saved to: $LOG_ARTIFACT_DIR"
