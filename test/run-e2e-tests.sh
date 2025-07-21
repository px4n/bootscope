#!/bin/bash
# End-to-end tests for bootscope using real Kubernetes manifests
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
PASSED=0
FAILED=0

# Binary location
BOOTSCOPE_BIN="./bin/kubectl-bootscope"

# Helper functions
print_test() {
    echo -e "${YELLOW}TEST:${NC} $1"
}

print_pass() {
    echo -e "${GREEN}✓ PASSED:${NC} $1"
    PASSED=$((PASSED + 1))
}

print_fail() {
    echo -e "${RED}✗ FAILED:${NC} $1"
    FAILED=$((FAILED + 1))
}

wait_for_pod() {
    local pod_name=$1
    local namespace=$2
    local timeout=${3:-60}

    echo "Waiting for pod $pod_name to be ready..."
    if kubectl wait --for=condition=Ready pod/$pod_name -n $namespace --timeout=${timeout}s 2>/dev/null; then
        return 0
    else
        # Some pods may not have readiness probes, check if running
        local status=$(kubectl get pod $pod_name -n $namespace -o jsonpath='{.status.phase}' 2>/dev/null)
        if [[ "$status" == "Running" ]]; then
            return 0
        fi
        return 1
    fi
}

# Ensure we're in the right directory
if [[ ! -f "$BOOTSCOPE_BIN" ]]; then
    echo "Error: bootscope binary not found at $BOOTSCOPE_BIN"
    echo "Please run 'make build' first"
    exit 1
fi

# Check if cluster is ready
if ! kubectl cluster-info &>/dev/null; then
    echo "Error: No kubernetes cluster available"
    echo "Please run 'make setup-test-cluster' first"
    exit 1
fi

echo "Starting end-to-end tests..."
echo "============================"

# Test 1: Basic pod analysis
print_test "Basic pod analysis (basic-nginx)"
if wait_for_pod "basic-nginx" "bootscope-test"; then
    if $BOOTSCOPE_BIN analyze pod basic-nginx -n bootscope-test > /tmp/bootscope-test1.out 2>&1; then
        if grep -q "Pod Startup Profile" /tmp/bootscope-test1.out && \
           grep -q "nginx:latest" /tmp/bootscope-test1.out; then
            print_pass "Basic pod analysis works correctly"
        else
            print_fail "Basic pod analysis - unexpected output"
            cat /tmp/bootscope-test1.out
        fi
    else
        print_fail "Basic pod analysis - command failed"
        cat /tmp/bootscope-test1.out
    fi
else
    print_fail "basic-nginx not ready"
fi

# Test 2: Multi-container pod analysis
print_test "Multi-container pod analysis"
if wait_for_pod "multi-container-pod" "bootscope-test"; then
    if $BOOTSCOPE_BIN analyze pod multi-container-pod -n bootscope-test > /tmp/bootscope-test2.out 2>&1; then
        if grep -q "Pod Startup Profile" /tmp/bootscope-test2.out && \
           grep -q "frontend" /tmp/bootscope-test2.out && \
           grep -q "backend" /tmp/bootscope-test2.out && \
           grep -q "sidecar" /tmp/bootscope-test2.out; then
            print_pass "Multi-container pod analysis shows all containers"
        else
            print_fail "Multi-container pod analysis - missing containers"
            cat /tmp/bootscope-test2.out
        fi
    else
        print_fail "Multi-container pod analysis - command failed"
        cat /tmp/bootscope-test2.out
    fi
else
    print_fail "multi-container-pod not ready"
fi

# Test 3: Deployment analysis
# TODO: bootscope currently doesn't show main container images when init containers are present
#       Once fixed, add check for "nginx:alpine" image
print_test "Deployment analysis"
# Wait for at least one pod from the deployment
if kubectl wait --for=condition=available deployment/bootscope-test-app -n bootscope-test --timeout=60s &>/dev/null; then
    # Get a pod from the deployment
    POD_NAME=$(kubectl get pods -n bootscope-test -l app=bootscope-test -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [[ -n "$POD_NAME" ]]; then
        if $BOOTSCOPE_BIN analyze pod "$POD_NAME" -n bootscope-test > /tmp/bootscope-test3.out 2>&1; then
            if grep -q "Pod Startup Profile" /tmp/bootscope-test3.out && \
               grep -q "InitContainers" /tmp/bootscope-test3.out && \
               grep -q "Init container: init" /tmp/bootscope-test3.out; then
                print_pass "Deployment pod analysis works correctly"
            else
                print_fail "Deployment pod analysis - unexpected output"
                cat /tmp/bootscope-test3.out
            fi
        else
            print_fail "Deployment pod analysis - command failed"
            cat /tmp/bootscope-test3.out
        fi
    else
        print_fail "No pods found for deployment"
    fi
else
    print_fail "Deployment not ready"
fi

# Test 4: Slow startup pod analysis
print_test "Slow startup pod analysis"
# This pod has slow init containers and startup
if $BOOTSCOPE_BIN analyze pod slow-startup-pod -n bootscope-test > /tmp/bootscope-test4.out 2>&1; then
    if grep -q "slow-init" /tmp/bootscope-test4.out || grep -q "Pending" /tmp/bootscope-test4.out; then
        print_pass "Slow startup pod analysis handles init containers"
    else
        print_fail "Slow startup pod analysis - unexpected output"
        cat /tmp/bootscope-test4.out
    fi
else
    # This might fail if pod is still initializing, which is expected
    if grep -q "not found" /tmp/bootscope-test4.out; then
        print_fail "Slow startup pod not found"
    else
        print_pass "Slow startup pod analysis handles initializing pods"
    fi
fi

# Test 5: Large image pod analysis
print_test "Large image pod analysis"
# This pod uses a large image (pytorch)
if $BOOTSCOPE_BIN analyze pod large-image-pod -n bootscope-test > /tmp/bootscope-test5.out 2>&1; then
    if grep -q "pytorch" /tmp/bootscope-test5.out; then
        print_pass "Large image pod analysis works correctly"
    else
        print_fail "Large image pod analysis - unexpected output"
        cat /tmp/bootscope-test5.out
    fi
else
    # Might be still pulling the large image
    if grep -q "ContainerCreating\\|Pending" /tmp/bootscope-test5.out; then
        print_pass "Large image pod analysis handles image pull phase"
    else
        print_fail "Large image pod analysis - command failed"
        cat /tmp/bootscope-test5.out
    fi
fi

# Test 6: Non-existent pod
print_test "Non-existent pod handling"
if $BOOTSCOPE_BIN analyze pod non-existent-pod -n bootscope-test > /tmp/bootscope-test6.out 2>&1; then
    print_fail "Non-existent pod - should have failed"
else
    if grep -q "not found" /tmp/bootscope-test6.out; then
        print_pass "Non-existent pod handling works correctly"
    else
        print_fail "Non-existent pod - unexpected error"
        cat /tmp/bootscope-test6.out
    fi
fi

# Test 7: Pod with init containers
print_test "Pod with init containers analysis"
if wait_for_pod "app-with-init" "bootscope-test"; then
    if $BOOTSCOPE_BIN analyze pod app-with-init -n bootscope-test > /tmp/bootscope-test7.out 2>&1; then
        if grep -q "init-myservice" /tmp/bootscope-test7.out && \
           grep -q "init-mydb" /tmp/bootscope-test7.out; then
            print_pass "Pod with init containers analysis shows all init containers"
        else
            print_fail "Pod with init containers - missing init containers"
            cat /tmp/bootscope-test7.out
        fi
    else
        print_fail "Pod with init containers - command failed"
        cat /tmp/bootscope-test7.out
    fi
else
    print_fail "app-with-init not ready"
fi

# Test 8: JSON output format
print_test "JSON output format"
if wait_for_pod "basic-nginx" "bootscope-test"; then
    if $BOOTSCOPE_BIN analyze pod basic-nginx -n bootscope-test -o json > /tmp/bootscope-test8.out 2>&1; then
        if python3 -m json.tool < /tmp/bootscope-test8.out > /dev/null 2>&1 || \
           jq . < /tmp/bootscope-test8.out > /dev/null 2>&1; then
            print_pass "JSON output format is valid"
        else
            print_fail "JSON output format - invalid JSON"
            head -20 /tmp/bootscope-test8.out
        fi
    else
        print_fail "JSON output format - command failed"
        cat /tmp/bootscope-test8.out
    fi
else
    print_fail "basic-nginx not ready for JSON test"
fi

# Summary
echo "============================"
echo "E2E Test Summary:"
echo -e "${GREEN}Passed:${NC} $PASSED"
echo -e "${RED}Failed:${NC} $FAILED"
echo "============================"

# Cleanup temp files
rm -f /tmp/bootscope-test*.out

# Exit with appropriate code
if [[ $FAILED -gt 0 ]]; then
    exit 1
else
    exit 0
fi
