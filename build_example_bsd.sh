#!/bin/bash
# build_bsd_test.sh - Test BSD cross-compilation with simple Go example
# Tests whether we can cross-compile pure Go code to BSD targets

set -e

BINARY_NAME="zen80_example"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=${VERSION}"
OUTPUT_DIR="build_test"
VERBOSE=false
DEBUG=false

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m'

echo -e "${BLUE}BSD Cross-Compilation Test${NC}"
echo -e "${BLUE}==========================${NC}"
echo -e "${CYAN}Testing pure Go cross-compilation to BSD targets${NC}"
echo ""

# BSD configurations
get_bsd_config() {
    local bsd=$1
    local field=$2
    
    case "${bsd}:${field}" in
        "freebsd:name") echo "FreeBSD" ;;
        "freebsd:desc") echo "Most Compatible" ;;
        "openbsd:name") echo "OpenBSD" ;;
        "openbsd:desc") echo "Security Focused" ;;
        "netbsd:name") echo "NetBSD" ;;
        "netbsd:desc") echo "Maximum Portability" ;;
        *) echo "" ;;
    esac
}

get_bsd_list() {
    echo "freebsd openbsd netbsd"
}

# Check prerequisites
check_prerequisites() {
    local error_count=0
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}❌ Docker is required but not installed${NC}"
        ((error_count++))
    fi

    if ! docker info >/dev/null 2>&1; then
        echo -e "${RED}❌ Docker is not running${NC}"
        ((error_count++))
    fi
    
    # Check for zen80 project structure
    if [ -f "go.mod" ]; then
        echo -e "${GREEN}✅ Found go.mod${NC}"
    else
        echo -e "${RED}❌ No go.mod found${NC}"
        ((error_count++))
    fi
    
    # Check for the example
    if [ -f "cmd/example/main.go" ]; then
        echo -e "${GREEN}✅ Found cmd/example/main.go${NC}"
        
        # Quick check - does it import CGO-dependent packages?
        if grep -q "github.com/gen2brain/raylib-go\|\"C\"" cmd/example/main.go; then
            echo -e "${YELLOW}⚠️  Example contains CGO dependencies${NC}"
        else
            echo -e "${GREEN}✅ Example appears to be pure Go${NC}"
        fi
        
        # Show what packages it imports
        echo -e "${CYAN}Example imports:${NC}"
        grep "^import\|^\s*\"" cmd/example/main.go | head -10 | sed 's/^/  /'
        
    else
        echo -e "${RED}❌ cmd/example/main.go not found${NC}"
        ((error_count++))
    fi
    
    if [ $error_count -gt 0 ]; then
        echo ""
        echo -e "${RED}Please fix the above errors before building.${NC}"
        exit 1
    fi
    
    echo -e "${CYAN}Working directory: $(pwd)${NC}"
    echo ""
}

# Create simple Dockerfile for pure Go cross-compilation
create_bsd_dockerfile() {
    local bsd=$1
    local goarch=$2
    local description=$(get_bsd_config "$bsd" "name")
    local full_binary_name="${BINARY_NAME}_${bsd}_${goarch}"
    
    cat > "Dockerfile.test.${bsd}.${goarch}" << EOF
# Simple Go cross-compilation test for ${description} ${goarch}
FROM golang:1.24 AS builder

# No CGO needed for pure Go
ENV CGO_ENABLED=0
ENV GOOS=${bsd}
ENV GOARCH=${goarch}

WORKDIR /app

# Copy the project
COPY . .

# Download dependencies
RUN go mod download

# Build the example with pure Go (no CGO) - using direct filename
RUN echo "Building pure Go example for ${bsd}/${goarch}..." && \\
    go build -v -ldflags="${LDFLAGS}" \\
        -o ${full_binary_name} ./cmd/example && \\
    echo "SUCCESS: Pure Go build completed" && \\
    ls -la ${full_binary_name} && \\
    echo "CGO_ENABLED=0,PURE_GO=true" > build_info_${bsd}_${goarch}

# Show some info about the binary
RUN file ${full_binary_name} || echo "file command not available"

# We'll extract directly from the builder stage, no need for scratch output stage
EOF
}

# Test build for specific BSD and architecture
build_bsd_test() {
    local bsd=$1
    local goarch=$2
    local description=$(get_bsd_config "$bsd" "desc")
    
    echo -e "${CYAN}Testing ${bsd}/${goarch}${NC} (${description})"
    
    local container_name="zen80-test-${bsd}-${goarch}"
    local dockerfile="Dockerfile.test.${bsd}.${goarch}"
    local build_success=false
    local full_binary_name="${BINARY_NAME}_${bsd}_${goarch}"
    
    create_bsd_dockerfile "$bsd" "$goarch"
    
    local build_log_file="build_test_${bsd}_${goarch}.log"
    
    if [ "$VERBOSE" = true ]; then
        echo -e "   ${BLUE}Building Docker image (verbose)...${NC}"
        if docker build -f "$dockerfile" -t "$container_name" . 2>&1 | tee "$build_log_file"; then
            build_success=true
        fi
    else
        echo -e "   ${BLUE}Building Docker image...${NC}"
        if docker build -f "$dockerfile" -t "$container_name" . >"$build_log_file" 2>&1; then
            build_success=true
        fi
    fi
    
    # Process build result
    if [ "$build_success" = true ]; then
        echo -e "   ${BLUE}Extracting binary...${NC}"
        
        # Create a temporary container to extract files from
        local temp_container="${container_name}-extract"
        
        # Clean up any existing container with this name first
        docker rm "$temp_container" >/dev/null 2>&1 || true
        
        echo -e "   ${BLUE}   Creating extraction container: ${temp_container}${NC}"
        if docker create --name "$temp_container" "$container_name" 2>&1; then
            
            # Extract binary from /app directory (builder stage path)
            echo -e "   ${BLUE}   Copying binary: /app/${full_binary_name}${NC}"
            if docker cp "${temp_container}:/app/${full_binary_name}" "$OUTPUT_DIR/" 2>&1; then
                echo -e "   ${GREEN}   ✅ Binary extracted successfully${NC}"
                
                # Also try to extract build info (optional)
                if docker cp "${temp_container}:/app/build_info_${bsd}_${goarch}" "$OUTPUT_DIR/" 2>/dev/null; then
                    echo -e "   ${CYAN}   ✅ Build info extracted${NC}"
                fi
                
                analyze_test_result "$bsd" "$goarch"
                local result=$?
                
                # Cleanup
                docker rm "$temp_container" >/dev/null 2>&1 || true
                if [ "$DEBUG" != true ]; then
                    docker rmi "$container_name" >/dev/null 2>&1 || true
                    rm -f "$build_log_file"
                fi
                rm -f "$dockerfile"
                
                return $result
            else
                echo -e "   ${RED}❌ Failed to extract binary from container${NC}"
                echo -e "   ${YELLOW}Docker cp error details:${NC}"
                docker cp "${temp_container}:/app/${full_binary_name}" "$OUTPUT_DIR/" 2>&1 | sed 's/^/     /'
                echo -e "   ${YELLOW}Container /app contents:${NC}"
                docker run --rm "$container_name" ls -la /app/ 2>/dev/null | sed 's/^/     /' || \
                    echo "     (unable to list container contents)"
            fi
            
            # Cleanup temp container
            docker rm "$temp_container" >/dev/null 2>&1 || true
        else
            echo -e "   ${RED}❌ Failed to create extraction container${NC}"
            echo -e "   ${YELLOW}Docker create error (retrying):${NC}"
            docker create --name "$temp_container" "$container_name" 2>&1 | sed 's/^/     /'
            echo -e "   ${YELLOW}Available Docker images:${NC}"
            docker images "$container_name" 2>/dev/null | sed 's/^/     /' || echo "     (no matching images found)"
        fi
    fi
    
    # Build or extraction failed
    echo -e "   ${RED}❌ Build failed for ${bsd}/${goarch}${NC}"
    
    if [ -f "$build_log_file" ]; then
        echo -e "   ${YELLOW}Error details (last 15 lines):${NC}"
        tail -15 "$build_log_file" | sed 's/^/     /'
        
        # Analyze common failure patterns in build logs
        local logs=$(cat "$build_log_file")
        if echo "$logs" | grep -qi "unsupported GOOS/GOARCH"; then
            echo -e "   ${YELLOW}   → Cause: Go doesn't support this GOOS/GOARCH combination${NC}"
        elif echo "$logs" | grep -qi "go build.*failed\|compilation terminated"; then
            echo -e "   ${YELLOW}   → Cause: Go build compilation error${NC}"
        elif echo "$logs" | grep -qi "cannot find package"; then
            echo -e "   ${YELLOW}   → Cause: Missing dependencies${NC}"
        elif echo "$logs" | grep -qi "build constraints exclude all"; then
            echo -e "   ${YELLOW}   → Cause: Build constraints prevent compilation${NC}"
        elif echo "$logs" | grep -qi "Failed to create extraction container"; then
            echo -e "   ${YELLOW}   → Cause: Docker container extraction issue${NC}"
        elif echo "$logs" | grep -qi "exporting layers.*done.*writing image.*done"; then
            echo -e "   ${YELLOW}   → Cause: Docker build succeeded, but extraction failed${NC}"
        elif echo "$logs" | grep -qi "go.mod\|go.sum" && echo "$logs" | grep -qi "error"; then
            echo -e "   ${YELLOW}   → Cause: Go module dependency issues${NC}"
        else
            echo -e "   ${YELLOW}   → Check full log: ${build_log_file}${NC}"
        fi
    fi
    
    # Cleanup on failure
    docker rm "${container_name}-extract" >/dev/null 2>&1 || true
    if [ "$DEBUG" != true ]; then
        docker rmi "$container_name" >/dev/null 2>&1 || true
    else
        echo -e "   ${BLUE}   Debug mode: Container ${container_name} preserved${NC}"
    fi
    rm -f "$dockerfile"
    
    return 1
}

# Analyze test build results
analyze_test_result() {
    local bsd=$1
    local goarch=$2
    local binary_path="$OUTPUT_DIR/${BINARY_NAME}_${bsd}_${goarch}"
    
    if [ ! -f "$binary_path" ]; then
        echo -e "   ${RED}❌ Binary not found at: ${binary_path}${NC}"
        echo -e "   ${YELLOW}Available files in output directory:${NC}"
        ls -la "$OUTPUT_DIR/" 2>/dev/null | sed 's/^/     /' || echo "     (directory empty or not found)"
        return 1
    fi
    
    local size=$(du -h "$binary_path" | cut -f1)
    local build_info_file="$OUTPUT_DIR/build_info_${bsd}_${goarch}"
    
    echo -e "   ${GREEN}✅ SUCCESS${NC}"
    echo -e "   ${GREEN}   → Pure Go cross-compilation works!${NC}"
    echo -e "   ${CYAN}   → Binary: ${BINARY_NAME}_${bsd}_${goarch} (${size})${NC}"
    
    # Try to get file type info if available
    if command -v file >/dev/null 2>&1; then
        local file_info=$(file "$binary_path" 2>/dev/null || echo "unknown")
        echo -e "   ${CYAN}   → Type: ${file_info}${NC}"
    fi
    
    if [ -f "$build_info_file" ]; then
        local build_info=$(cat "$build_info_file")
        echo -e "   ${CYAN}   → Info: ${build_info}${NC}"
        rm -f "$build_info_file"
    fi
    
    return 0
}

# Test all BSD targets for an architecture (sequential to avoid race conditions)
test_arch() {
    local goarch=$1
    
    echo -e "${BLUE}Testing all BSDs for ${goarch} architecture...${NC}"
    echo ""
    
    local success_count=0
    local total_count=$(echo $(get_bsd_list) | wc -w)
    
    # Build sequentially to avoid Docker race conditions and better error reporting
    for bsd in $(get_bsd_list); do
        if build_bsd_test "$bsd" "$goarch"; then
            ((success_count++))
        fi
        echo ""
    done
    
    echo -e "${BLUE}${goarch} Test Summary: ${GREEN}${success_count}${NC}/${total_count} successful${NC}"
    echo ""
    
    return $((total_count - success_count))
}

# Show usage help
show_help() {
    echo "Usage: $0 [OPTIONS] [BSD] [ARCH]"
    echo ""
    echo "Test BSD cross-compilation with simple zen80 example"
    echo ""
    echo "BSD Options:"
    for bsd in $(get_bsd_list); do
        local name=$(get_bsd_config "$bsd" "name")
        printf "  %-10s %s\n" "$bsd" "$name"
    done
    echo ""
    echo "Architecture Options:"
    echo "  amd64          x86-64"
    echo "  arm64          ARM64" 
    echo "  386            x86-32"
    echo ""
    echo "Examples:"
    echo "  $0                        # Test all BSD/arch combinations"
    echo "  $0 freebsd amd64         # Test FreeBSD x86-64 only"
    echo "  $0 all amd64             # Test all BSDs for x86-64"
    echo ""
    echo "Options:"
    echo "  -h, --help               Show this help"
    echo "  -v, --verbose            Show Docker build output"
    echo "  -d, --debug              Debug mode (preserve containers)"
    echo ""
    echo "Purpose:"
    echo "  This script tests whether BSD cross-compilation works at all"
    echo "  by building the simple cmd/example/main.go without CGO dependencies."
    echo "  If this fails, the BSD toolchain setup has issues."
    echo "  If this succeeds, the ZenZX build issues are CGO/raylib related."
}

# Main execution
main() {
    check_prerequisites
    mkdir -p "$OUTPUT_DIR"
    
    local bsd_target=""
    local arch_target=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                echo -e "${BLUE}Verbose mode enabled${NC}"
                shift
                ;;
            -d|--debug)
                DEBUG=true
                VERBOSE=true
                echo -e "${BLUE}Debug mode enabled${NC}"
                shift
                ;;
            freebsd|openbsd|netbsd|all)
                bsd_target="$1"
                shift
                ;;
            amd64|arm64|386|all)
                arch_target="$1"
                shift
                ;;
            *)
                echo -e "${RED}Unknown option: $1${NC}"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Set defaults
    bsd_target=${bsd_target:-"all"}
    arch_target=${arch_target:-"all"}
    
    local start_time=$(date +%s)
    local total_failures=0
    
    echo -e "${PURPLE}Starting BSD Cross-Compilation Test...${NC}"
    echo ""
    
    # Execute tests based on targets
    if [[ "$bsd_target" == "all" && "$arch_target" == "all" ]]; then
        # Test everything
        for arch in amd64 arm64 386; do
            test_arch "$arch"
            total_failures=$((total_failures + $?))
        done
        
    elif [[ "$bsd_target" == "all" ]]; then
        # All BSDs, specific architecture
        test_arch "$arch_target"
        total_failures=$?
        
    elif [[ "$arch_target" == "all" ]]; then
        # Specific BSD, all architectures
        echo -e "${BLUE}Testing ${bsd_target} for all architectures...${NC}"
        echo ""
        
        for arch in amd64 arm64 386; do
            if build_bsd_test "$bsd_target" "$arch"; then
                echo ""
            else
                ((total_failures++))
            fi
        done
        
    else
        # Specific BSD and architecture
        if ! build_bsd_test "$bsd_target" "$arch_target"; then
            total_failures=1
        fi
    fi
    
    # Final summary
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    echo -e "${BLUE}=== Test Complete ===${NC}"
    echo -e "${CYAN}Duration: ${duration}s${NC}"
    
    if [ $total_failures -eq 0 ]; then
        echo -e "${GREEN}🎉 All tests successful!${NC}"
        echo -e "${GREEN}✅ BSD cross-compilation environment is working${NC}"
        echo -e "${CYAN}→ ZenZX build issues are likely CGO/raylib-go related${NC}"
    else
        echo -e "${YELLOW}${total_failures} test(s) failed${NC}"
        echo -e "${YELLOW}⚠️  BSD cross-compilation environment has issues${NC}"
    fi
    
    # Show results
    if ls "$OUTPUT_DIR"/${BINARY_NAME}_*_* 1> /dev/null 2>&1; then
        echo ""
        echo -e "${CYAN}Test binaries created:${NC}"
        ls -lh "$OUTPUT_DIR"/${BINARY_NAME}_*_* | while read line; do
            echo "  $line"
        done
        
        echo ""
        echo -e "${BLUE}Next Steps:${NC}"
        if [ $total_failures -eq 0 ]; then
            echo -e "• These binaries should run on actual BSD systems"
            echo -e "• The issue with ZenZX is CGO/raylib-go cross-compilation"
            echo -e "• Consider building ZenZX natively on BSD systems"
            echo -e "• Or investigate CGO-free graphics alternatives"
        else
            echo -e "• Fix the basic Go cross-compilation issues first"
            echo -e "• Check Go version compatibility with BSD targets"
            echo -e "• Verify Docker environment setup"
        fi
    fi
    
    # Show debug info if failures occurred
    if [ $total_failures -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}Debug Information:${NC}"
        if ls build_test_*_*.log 1> /dev/null 2>&1; then
            echo -e "• Build logs saved:"
            ls build_test_*_*.log | sed 's/^/  /'
        fi
        if [ "$DEBUG" = true ]; then
            echo -e "• Docker containers preserved for inspection"
        fi
        echo -e "• Run with --verbose for real-time build output"
    fi
    
    exit $total_failures
}

# Run main with all arguments
main "$@"