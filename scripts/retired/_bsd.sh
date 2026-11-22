#!/bin/bash
# build_bsd_improved.sh - Improved BSD build script for ZenZX
# Targets: FreeBSD, OpenBSD, NetBSD
# Architectures: amd64, arm64
# Compatible with bash 3.2+ (macOS default)

set -e

# Check bash version compatibility
if [ -z "$BASH_VERSION" ]; then
    echo "Error: This script requires bash (you may be using sh or dash)"
    echo "Try: bash ./build_bsd_improved.sh"
    exit 1
fi

BINARY_NAME="zenzx"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=${VERSION}"
OUTPUT_DIR="build"
MAX_PARALLEL=3
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

echo -e "${BLUE}ZenZX Improved BSD Build Script${NC}"
echo -e "${BLUE}===============================${NC}"
echo ""
echo -e "${CYAN}Tip: If builds fail, try:${NC}"
echo -e "  ${0} --verbose freebsd amd64    # See real-time build output"
echo -e "  ${0} --debug freebsd amd64      # Preserve containers + full logs"
echo ""

# BSD configurations (compatible with bash 3.2+)
get_bsd_config() {
    local bsd=$1
    local field=$2
    
    case "${bsd}:${field}" in
        "freebsd:name") echo "FreeBSD" ;;
        "freebsd:desc") echo "Most Compatible" ;;
        "freebsd:compiler") echo "clang" ;;
        "openbsd:name") echo "OpenBSD" ;;
        "openbsd:desc") echo "Security Focused" ;;
        "openbsd:compiler") echo "clang" ;;
        "netbsd:name") echo "NetBSD" ;;
        "netbsd:desc") echo "Maximum Portability" ;;
        "netbsd:compiler") echo "gcc" ;;
        *) echo "" ;;
    esac
}

# Get list of supported BSDs
get_bsd_list() {
    echo "freebsd openbsd netbsd"
}

# Architecture-specific build flags
get_arch_flags() {
    local bsd=$1
    local goarch=$2
    
    case "${bsd}/${goarch}" in
        "openbsd/amd64")
            echo "-buildmode=pie"  # OpenBSD prefers PIE
            ;;
        "freebsd/arm64"|"netbsd/arm64")
            echo "-tags=netgo"     # Better networking on ARM64
            ;;
        *)
            echo ""
            ;;
    esac
}

# Check prerequisites and find source directory
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
    
    # Check for zen80 project structure with zenzx subproject
    if [ -f "go.mod" ] && [ -d "zenzx" ] && [ -f "zenzx/zenzx.go" ]; then
        echo -e "${GREEN}✅ Found zen80 project with zenzx subproject${NC}"
        
        # Check for essential source files in zenzx subdirectory
        local essential_files="display.go memory.go io.go audio_oto.go"
        local missing_files=0
        
        for file in $essential_files; do
            if ! [ -f "zenzx/$file" ]; then
                echo -e "${YELLOW}⚠️  Warning: zenzx/$file not found${NC}"
                ((missing_files++))
            fi
        done
        
        if [ $missing_files -eq 0 ]; then
            echo -e "${GREEN}✅ All essential ZenZX source files found${NC}"
        else
            echo -e "${YELLOW}⚠️  $missing_files essential files missing${NC}"
        fi
        
        # Check for zen80 dependency
        if [ -d "z80" ] || [ -d "zen80" ] || grep -q "zen80" go.mod 2>/dev/null; then
            echo -e "${GREEN}✅ Found zen80 dependency${NC}"
        else
            echo -e "${YELLOW}⚠️  zen80 dependency not clearly found${NC}"
        fi
        
    else
        echo -e "${RED}❌ Expected zen80 project structure not found${NC}"
        echo -e "${YELLOW}Expected structure:${NC}"
        echo -e "  current_dir/          (you are here)"
        echo -e "  ├── go.mod           (main project go.mod)"
        echo -e "  ├── go.sum           (dependencies)"
        echo -e "  ├── zen80/           (Z80 emulator code)"
        echo -e "  └── zenzx/           (ZenZX subproject)"
        echo -e "      ├── zenzx.go"
        echo -e "      └── *.go files"
        echo -e "${CYAN}Current directory: $(pwd)${NC}"
        echo -e "${CYAN}Contents: $(ls -la)${NC}"
        ((error_count++))
    fi
    
    if [ $error_count -gt 0 ]; then
        echo ""
        echo -e "${RED}Please fix the above errors before building.${NC}"
        exit 1
    fi
    
    # Show current directory for confirmation
    echo -e "${CYAN}Working directory: $(pwd)${NC}"
    echo -e "${CYAN}ZenZX source: $(pwd)/zenzx/${NC}"
    echo ""
}

# Create optimized multi-stage Dockerfile
create_bsd_dockerfile() {
    local bsd=$1
    local goarch=$2
    local description=$(get_bsd_config "$bsd" "name")
    local compiler=$(get_bsd_config "$bsd" "compiler")
    local arch_flags=$(get_arch_flags $bsd $goarch)
    
    cat > "Dockerfile.${bsd}" << EOF
# Multi-stage build for ${description} ${goarch}
FROM golang:1.24 as builder

# Install build dependencies
RUN apt-get update -qq && \\
    apt-get install -y -qq --no-install-recommends \\
        build-essential \\
        ${compiler} \\
        pkg-config \\
        libgl1-mesa-dev \\
        libxi-dev \\
        libxcursor-dev \\
        libxrandr-dev \\
        libxinerama-dev \\
        libasound2-dev && \\
    apt-get clean && \\
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the entire project (zen80 + zenzx subproject)
COPY . .

# Download dependencies (go.mod is at project root)
RUN go mod download

# Set build environment
ENV CGO_ENABLED=1
ENV GOOS=${bsd}
ENV GOARCH=${goarch}
ENV CC=${compiler}
ENV CXX=${compiler}++

# Build ZenZX subproject with CGO first, fallback to static
RUN echo "Attempting CGO build for zenzx package..." && \\
    go build -v -ldflags="${LDFLAGS}" ${arch_flags} \\
        -o \${BINARY_NAME}_${bsd}_${goarch}_cgo ./zenzx && \\
    echo "CGO_ENABLED=1" > build_info_${bsd}_${goarch} || \\
    (echo "CGO build failed, trying static build..." && \\
     CGO_ENABLED=0 go build -v -ldflags="${LDFLAGS}" ${arch_flags} \\
        -o \${BINARY_NAME}_${bsd}_${goarch}_static ./zenzx && \\
     echo "CGO_ENABLED=0" > build_info_${bsd}_${goarch})

# Determine which binary was successfully built
RUN if [ -f "\${BINARY_NAME}_${bsd}_${goarch}_cgo" ]; then \\
        mv \${BINARY_NAME}_${bsd}_${goarch}_cgo \${BINARY_NAME}_${bsd}_${goarch}; \\
    else \\
        mv \${BINARY_NAME}_${bsd}_${goarch}_static \${BINARY_NAME}_${bsd}_${goarch}; \\
    fi

# Minimal output stage
FROM scratch as output
COPY --from=builder /app/\${BINARY_NAME}_${bsd}_${goarch} /
COPY --from=builder /app/build_info_${bsd}_${goarch} /
EOF
}

# Analyze build result
analyze_build_result() {
    local bsd=$1
    local goarch=$2
    local binary_path="$OUTPUT_DIR/${BINARY_NAME}_${bsd}_${goarch}"
    
    if [ ! -f "$binary_path" ]; then
        echo -e "   ${RED}❌ Binary not found${NC}"
        return 1
    fi
    
    local size=$(du -h "$binary_path" | cut -f1)
    local build_info_file="$OUTPUT_DIR/build_info_${bsd}_${goarch}"
    
    if [ -f "$build_info_file" ]; then
        local cgo_status=$(cat "$build_info_file")
        if [[ "$cgo_status" == "CGO_ENABLED=1" ]]; then
            echo -e "   ${GREEN}✅ Success with CGO${NC} (full functionality)"
        else
            echo -e "   ${YELLOW}⚠️  Success without CGO${NC} (limited graphics/audio)"
        fi
        rm -f "$build_info_file"
    else
        echo -e "   ${GREEN}✅ Success${NC} (status unknown)"
    fi
    
    echo -e "   ${CYAN}Binary: ${BINARY_NAME}_${bsd}_${goarch} (${size})${NC}"
    return 0
}

# Enhanced error analysis with actual build logs
analyze_build_failure() {
    local bsd=$1
    local goarch=$2
    local build_log_file="build_${bsd}_${goarch}.log"
    
    echo -e "   ${RED}❌ Build failed for ${bsd}/${goarch}${NC}"
    
    # Show the actual Docker build output if log file exists
    if [ -f "$build_log_file" ]; then
        echo -e "   ${YELLOW}Docker build output (last 20 lines):${NC}"
        tail -20 "$build_log_file" | sed 's/^/     /'
        
        # Try to get specific error information from build log
        local logs=$(cat "$build_log_file")
        if echo "$logs" | grep -q "undefined reference\|undefined symbol"; then
            echo -e "   ${YELLOW}   → Likely cause: Missing system libraries${NC}"
        elif echo "$logs" | grep -q "unsupported GOOS/GOARCH"; then
            echo -e "   ${YELLOW}   → Likely cause: Architecture not supported${NC}"
        elif echo "$logs" | grep -q "cgo: C compiler"; then
            echo -e "   ${YELLOW}   → Likely cause: C compiler issues${NC}"
        elif echo "$logs" | grep -q "go.mod\|go.sum"; then
            echo -e "   ${YELLOW}   → Likely cause: Go module issues${NC}"
        elif echo "$logs" | grep -q "COPY failed\|ADD failed"; then
            echo -e "   ${YELLOW}   → Likely cause: File copy issues in Dockerfile${NC}"
        elif echo "$logs" | grep -q "apt-get\|package"; then
            echo -e "   ${YELLOW}   → Likely cause: Package installation failed${NC}"
        else
            echo -e "   ${YELLOW}   → Check full log: ${build_log_file}${NC}"
        fi
        
        echo -e "   ${CYAN}   Full logs saved to: ${build_log_file}${NC}"
    else
        echo -e "   ${YELLOW}   → No build log available${NC}"
    fi
}

# Build for specific BSD and architecture
build_bsd() {
    local bsd=$1
    local goarch=$2
    local description=$(get_bsd_config "$bsd" "desc")
    
    echo -e "${CYAN}Building ${bsd}/${goarch}${NC} (${description})"
    
    local container_name="zenzx-${bsd}-${goarch}-builder"
    local dockerfile="Dockerfile.${bsd}"
    local build_success=false
    
    # Create BSD-specific Dockerfile
    create_bsd_dockerfile "$bsd" "$goarch"
    
    # Set up build output redirection and log file
    local build_log_file="build_${bsd}_${goarch}.log"
    
    if [ "$VERBOSE" = true ]; then
        echo -e "   ${BLUE}Building Docker image...${NC}"
        # In verbose mode, show output in real-time AND save to log
        if docker build -f "$dockerfile" -t "$container_name" . 2>&1 | tee "$build_log_file"; then
            build_success=true
        else
            build_success=false
        fi
    else
        # Silent mode - just save to log file
        if docker build -f "$dockerfile" -t "$container_name" . >"$build_log_file" 2>&1; then
            build_success=true
        else
            build_success=false
        fi
    fi
    
    # Process build result
    if [ "$build_success" = true ]; then
        # Extract binaries and build info
        if docker create --name "${container_name}-extract" "$container_name" >/dev/null 2>&1; then
            if docker cp "${container_name}-extract:/${BINARY_NAME}_${bsd}_${goarch}" "$OUTPUT_DIR/" 2>/dev/null && \
               docker cp "${container_name}-extract:/build_info_${bsd}_${goarch}" "$OUTPUT_DIR/" 2>/dev/null; then
                
                analyze_build_result "$bsd" "$goarch"
                local result=$?
                
                # Cleanup
                docker rm "${container_name}-extract" >/dev/null 2>&1 || true
                if [ "$DEBUG" != true ]; then
                    docker rmi "$container_name" >/dev/null 2>&1 || true
                    rm -f "$build_log_file"  # Remove log file on success unless debug mode
                fi
                rm -f "$dockerfile"
                
                return $result
            fi
            docker rm "${container_name}-extract" >/dev/null 2>&1 || true
        fi
    fi
    
    # Build failed - show detailed error info
    analyze_build_failure "$bsd" "$goarch"
    
    # Cleanup on failure (but keep logs and optionally containers for debugging)
    docker rm "${container_name}-extract" >/dev/null 2>&1 || true
    if [ "$DEBUG" != true ]; then
        docker rmi "$container_name" >/dev/null 2>&1 || true
    else
        echo -e "   ${BLUE}   Debug mode: Container ${container_name} preserved for inspection${NC}"
    fi
    rm -f "$dockerfile"
    
    return 1
}

# Build all targets for an architecture in parallel
build_arch_parallel() {
    local goarch=$1
    local pids=()
    local results=()
    
    echo -e "${BLUE}Building all BSDs for ${goarch} architecture...${NC}"
    echo ""
    
    # Start parallel builds
    for bsd in $(get_bsd_list); do
        build_bsd "$bsd" "$goarch" &
        pids+=($!)
        results+=("${bsd}/${goarch}")
    done
    
    # Wait for all builds and collect results
    local success_count=0
    local total_count=$(echo $(get_bsd_list) | wc -w)
    
    for i in "${!pids[@]}"; do
        local pid=${pids[$i]}
        local target=${results[$i]}
        
        if wait $pid; then
            ((success_count++))
        fi
    done
    
    echo ""
    echo -e "${BLUE}${goarch} Summary: ${GREEN}${success_count}${NC}/${total_count} successful${NC}"
    echo ""
    
    return $((total_count - success_count))
}

# Show usage help
show_help() {
    echo "Usage: $0 [OPTIONS] [BSD] [ARCH]"
    echo ""
    echo "Build ZenZX for BSD systems with optimized Docker builds"
    echo ""
    echo "BSD Options:"
    for bsd in $(get_bsd_list); do
        local name=$(get_bsd_config "$bsd" "name")
        local desc=$(get_bsd_config "$bsd" "desc")
        printf "  %-10s %s (%s)\n" "$bsd" "$name" "$desc"
    done
    echo ""
    echo "Architecture Options:"
    echo "  amd64          x86-64 (most common)"
    echo "  arm64          ARM64 (growing popularity)"
    echo ""
    echo "Examples:"
    echo "  $0                        # Build all BSD/arch combinations"
    echo "  $0 freebsd amd64         # Build FreeBSD for x86-64"
    echo "  $0 all amd64             # Build all BSDs for x86-64"
    echo "  $0 freebsd all           # Build FreeBSD for all architectures"
    echo ""
    echo "Options:"
    echo "  -h, --help               Show this help"
    echo "  -v, --verbose            Show Docker build output"
    echo "  -d, --debug              Debug mode (preserve containers, save logs)"
    echo "  -j N                     Set parallel build limit (default: $MAX_PARALLEL)"
    echo ""
    echo "Debug Information:"
    echo "  On failure, logs are saved as: build_{bsd}_{arch}.log"
    echo "  Use --verbose to see real-time build output"
    echo "  Use --debug to preserve Docker containers for inspection"
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
                VERBOSE=true  # Debug implies verbose
                echo -e "${BLUE}Debug mode enabled (containers preserved, full logs)${NC}"
                shift
                ;;
            -j)
                MAX_PARALLEL="$2"
                shift 2
                ;;
            freebsd|openbsd|netbsd|all)
                bsd_target="$1"
                shift
                ;;
            amd64|arm64|all)
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
    
    # Execute builds based on targets
    if [[ "$bsd_target" == "all" && "$arch_target" == "all" ]]; then
        # Build everything
        echo -e "${PURPLE}Building for all BSD systems and architectures...${NC}"
        echo ""
        
        for arch in amd64 arm64; do
            build_arch_parallel "$arch"
            total_failures=$((total_failures + $?))
        done
        
    elif [[ "$bsd_target" == "all" ]]; then
        # All BSDs, specific architecture
        build_arch_parallel "$arch_target"
        total_failures=$?
        
    elif [[ "$arch_target" == "all" ]]; then
        # Specific BSD, all architectures
        echo -e "${BLUE}Building ${bsd_target} for all architectures...${NC}"
        echo ""
        
        for arch in amd64 arm64; do
            if build_bsd "$bsd_target" "$arch"; then
                echo ""
            else
                ((total_failures++))
            fi
        done
        
    else
        # Specific BSD and architecture
        if ! build_bsd "$bsd_target" "$arch_target"; then
            total_failures=1
        fi
    fi
    
    # Final summary
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    echo -e "${BLUE}=== Build Complete ===${NC}"
    echo -e "${CYAN}Duration: ${duration}s${NC}"
    
    if [ $total_failures -eq 0 ]; then
        echo -e "${GREEN}All builds successful!${NC}"
    else
        echo -e "${YELLOW}${total_failures} build(s) failed${NC}"
    fi
    
    if ls "$OUTPUT_DIR"/${BINARY_NAME}_*_* 1> /dev/null 2>&1; then
        echo ""
        echo -e "${CYAN}Built binaries:${NC}"
        ls -lh "$OUTPUT_DIR"/${BINARY_NAME}_*_* | while read line; do
            echo "  $line"
        done
    fi
    
    echo ""
    echo -e "${BLUE}BSD Compatibility Notes:${NC}"
    echo -e "• ${GREEN}CGO builds${NC}: Full graphics and audio support"
    echo -e "• ${YELLOW}Static builds${NC}: Limited functionality, no audio/graphics"
    echo -e "• Test on actual BSD systems for production use"
    
    # Show log files if any failures occurred
    if [ $total_failures -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}Debug Information:${NC}"
        if ls build_*_*.log 1> /dev/null 2>&1; then
            echo -e "• Build logs saved:"
            ls build_*_*.log | sed 's/^/  /'
        fi
        if [ "$DEBUG" = true ]; then
            echo -e "• Docker containers preserved for inspection"
            echo -e "  Use: docker ps -a | grep zenzx"
        fi
        echo -e "• Run with --verbose to see real-time output"
    fi
    
    exit $total_failures
}

# Run main with all arguments
main "$@"