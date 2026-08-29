#!/bin/bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || cat VERSION 2>/dev/null || echo dev)
mkdir -p build

echo "Compilando ZenZX para Linux..."
docker run --rm --platform linux/amd64 -v $(pwd):/app -w /app golang:1.25 \
  sh -c "
    apt-get update -qq && \
    apt-get install -y -qq build-essential \
      libgl1-mesa-dev \
      libxi-dev \
      libxcursor-dev \
      libxrandr-dev \
      libxinerama-dev \
      libasound2-dev \
      libwayland-dev \
      libxkbcommon-dev \
      wayland-protocols && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o zenzx_linux -v -ldflags=\"-s -w -X main.version=${VERSION}\" .
  "

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Compilación exitosa!"
    echo ""
    echo "Detalles del binario:"
    ls -lh zenzx_linux
    echo ""
    echo "Tipo de archivo:"
    file zenzx_linux
    echo ""
    echo "Tamaño legible:"
    du -h zenzx_linux
else
    echo ""
    echo "❌ Error en la compilación"
    exit 1
fi
