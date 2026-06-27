# Multi-stage build for  
FROM golang:1.24 as builder

# Install build dependencies
RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends \
        build-essential \
         \
        pkg-config \
        libgl1-mesa-dev \
        libxi-dev \
        libxcursor-dev \
        libxrandr-dev \
        libxinerama-dev \
        libasound2-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the entire project (both zen80 and zenzx)
COPY . .

# Change to zenzx subdirectory
WORKDIR /app/zenzx

# Download dependencies
RUN go mod download

# Set build environment
ENV CGO_ENABLED=1
ENV GOOS=
ENV GOARCH=
ENV CC=
ENV CXX=++

# Build with CGO first, fallback to static
RUN echo "Attempting CGO build..." && \
    go build -v -ldflags=""  \
        -o ${BINARY_NAME}___cgo ./ && \
    echo "CGO_ENABLED=1" > build_info__ || \
    (echo "CGO build failed, trying static build..." && \#!/bin/bash
# build_bsd_improved.sh - Improved BSD build script for ZenZX
# Targets: FreeBSD, OpenBSD, NetBSD
# Architectures: amd64, arm64
# Compatible with bash 3.2+ (macOS default)

set -e

# Check bash version compatibility
if [ -z "3.2.57(1)-release" ]; then
    echo "Error: This script requires bash (you may be using sh or dash)"
    echo "Try: bash ./build_bsd_improved.sh"
    exit 1
fi

BINARY_NAME="zenzx"
VERSION=b5b712c-dirty
LDFLAGS="-s -w -X main.version="
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

echo -e "ZenZX Improved BSD Build Script"
echo -e "==============================="
echo ""
echo -e "Tip: If builds fail, try:"
echo -e "  ./build_bsd.sh --verbose freebsd amd64    # See real-time build output"
echo -e "  ./build_bsd.sh --debug freebsd amd64      # Preserve containers + full logs"
echo ""

# Check prerequisites and find source directory
check_prerequisites() {
    local error_count=0
    
    if ! command -v docker &> /dev/null; then
        echo -e "❌ Docker is required but not installed"
        ((error_count++))
    fi

    if ! docker info >/dev/null 2>&1; then
        echo -e "❌ Docker is not running"
        ((error_count++))
    fi
    
    # Check for ZenZX subproject structure
    if [ -d "zenzx" ] && [ -f "zenzx/zenzx.go" ] && [ -f "zenzx/go.mod" ]; then
        echo -e "✅ Found ZenZX source in zenzx/ subdirectory"
        
        # Check for essential source files in zenzx subdirectory
        local essential_files="display.go memory.go io.go audio_oto.go"
        local missing_files=0
        
        for file in ; do
            if ! [ -f "zenzx/" ]; then
                echo -e "⚠️  Warning: zenzx/ not found"
                ((missing_files++))
            fi
        done
        
        if [  -eq 0 ]; then
            echo -e "✅ All essential source files found"
        else
            echo -e "⚠️   essential files missing"
        fi
        
        # Check for zen80 dependency if it exists
        if [ -d "zen80" ]; then
            echo -e "✅ Found zen80 dependency"
        else
            echo -e "⚠️  zen80 directory not found (may be ok if using external module)"
        fi
        
    else
        echo -e "❌ ZenZX source not found"
        echo -e "Expected structure:"
        echo -e "  current_dir/"
        echo -e "  ├── zen80/           (Z80 emulator)"
        echo -e "  └── zenzx/           (ZenZX subproject)"
        echo -e "      ├── zenzx.go"
        echo -e "      ├── go.mod"
        echo -e "      └── *.go files"
        echo -e "Current directory: /Users/haitch/prj/repo/zen80"
        ((error_count++))
    fi
    
    if [  -gt 0 ]; then
        echo ""
        echo -e "Please fix the above errors before building."
        exit 1
    fi
    
    # Show current directory for confirmation
    echo -e "Working directory: /Users/haitch/prj/repo/zen80"
    echo -e "ZenZX source: /Users/haitch/prj/repo/zen80/zenzx/"
    echo ""
}dev/null 2>&1; then
        echo -e "❌ Docker is not running"
        ((error_count++))
    fi

    if ! [ -f "go.mod" ]; then
        echo -e "❌ go.mod not found. Run from project root directory."
        ((error_count++))
    fi
    
    if ! [ -f "zenzx.go" ]; then
        echo -e "❌ zenzx.go not found. Are you in the ZenZX project directory?"
        ((error_count++))
    fi
    
    # Check for essential source files
    local essential_files="display.go memory.go io.go audio_oto.go"
    for file in ; do
        if ! [ -f "" ]; then
            echo -e "⚠️  Warning:  not found"
        fi
    done
    
    if [  -gt 0 ]; then
        echo ""
        echo -e "Please fix the above errors before building."
        exit 1
    fi
    
    # Show current directory for confirmation
    echo -e "✅ Prerequisites check passed"
    echo -e "Working directory: /Users/haitch/prj/repo/zen80"
    echo ""
}

# BSD configurations (compatible with bash 3.2+)
get_bsd_config() {
    local bsd=
    local field=
    
    case ":" in
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
    local bsd=
    local goarch=
    
    case "/" in
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

# Create optimized multi-stage Dockerfile
create_bsd_dockerfile() {
    local bsd=
    local goarch=
    local description=
    local compiler=
    local arch_flags=
    
    cat > "Dockerfile." << EOF
# Multi-stage build for  
FROM golang:1.24 as builder

# Install build dependencies
RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends \
        build-essential \
         \
        pkg-config \
        libgl1-mesa-dev \
        libxi-dev \
        libxcursor-dev \
        libxrandr-dev \
        libxinerama-dev \
        libasound2-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the entire project (zen80 + zenzx subproject)
COPY . .

# Download dependencies (go.mod is at project root)
RUN go mod download

# Set build environment
ENV CGO_ENABLED=1
ENV GOOS=
ENV GOARCH=
ENV CC=
ENV CXX=++

# Build ZenZX subproject with CGO first, fallback to static
RUN echo "Attempting CGO build for zenzx package..." && \
    go build -v -ldflags=""  \
        -o ${BINARY_NAME}___cgo ./zenzx && \
    echo "CGO_ENABLED=1" > build_info__ || \
    (echo "CGO build failed, trying static build..." && \
     CGO_ENABLED=0 go build -v -ldflags=""  \
        -o ${BINARY_NAME}___static ./zenzx && \
     echo "CGO_ENABLED=0" > build_info__)

# Determine which binary was successfully built
RUN if [ -f "${BINARY_NAME}___cgo" ]; then \
        mv ${BINARY_NAME}___cgo ${BINARY_NAME}__; \
    else \
        mv ${BINARY_NAME}___static ${BINARY_NAME}__; \
    fi

# Minimal output stage
FROM scratch as output
COPY --from=builder /app/${BINARY_NAME}__ /
COPY --from=builder /app/build_info__ /
